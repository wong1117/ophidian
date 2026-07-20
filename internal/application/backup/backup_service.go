package backup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type BackupStatus string

const (
	StatusRunning   BackupStatus = "RUNNING"
	StatusCompleted BackupStatus = "COMPLETED"
	StatusFailed    BackupStatus = "FAILED"
)

type BackupMetadata struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Status       BackupStatus `json:"status"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	Size         int64        `json:"size"`
	EventCount   int          `json:"event_count"`
	FromVersion  int          `json:"from_version"`
	ToVersion    int          `json:"to_version"`
	Checksum     string       `json:"checksum"`
	Aggregates   []string     `json:"aggregates"`
	Error        string       `json:"error,omitempty"`
}

type BackupRecord struct {
	Metadata BackupMetadata    `json:"metadata"`
	Events   []json.RawMessage `json:"events"`
}

type EventStreamReader interface {
	LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]interface{}, error)
	LoadAllEvents(ctx context.Context, from, to time.Time) ([]interface{}, error)
}

type BackupStore interface {
	Save(ctx context.Context, meta BackupMetadata, data []byte) error
	Load(ctx context.Context, backupID string) ([]byte, error)
	List(ctx context.Context) ([]BackupMetadata, error)
	Delete(ctx context.Context, backupID string) error
}

type AuditLogger interface {
	Log(ctx context.Context, action string, details map[string]interface{})
}

type Metrics interface {
	Increment(name string)
	RecordDuration(name string, d time.Duration)
}

type BackupService struct {
	eventStore EventStreamReader
	store      BackupStore
	audit      AuditLogger
	metrics    Metrics
}

func NewBackupService(eventStore EventStreamReader, store BackupStore) *BackupService {
	return &BackupService{
		eventStore: eventStore,
		store:      store,
	}
}

func (s *BackupService) WithAudit(audit AuditLogger) *BackupService {
	s.audit = audit
	return s
}

func (s *BackupService) WithMetrics(m Metrics) *BackupService {
	s.metrics = m
	return s
}

func (s *BackupService) FullBackup(ctx context.Context) (*BackupMetadata, error) {
	now := time.Now().UTC()
	from := time.Time{}

	meta := &BackupMetadata{
		ID:        fmt.Sprintf("backup-%s", now.Format("20060102-150405")),
		Type:      "FULL",
		Status:    StatusRunning,
		StartedAt: now,
	}

	s.recordMetric("backup.started")
	start := time.Now()

	events, err := s.eventStore.LoadAllEvents(ctx, from, now)
	if err != nil {
		meta.Status = StatusFailed
		meta.Error = err.Error()
		s.recordMetric("backup.failed")
		return meta, fmt.Errorf("full backup: %w", err)
	}

	if err := s.saveBackup(ctx, meta, events, 0); err != nil {
		meta.Status = StatusFailed
		meta.Error = err.Error()
		s.recordMetric("backup.failed")
		return meta, err
	}

	s.recordMetric("backup.completed")
	s.recordDuration("backup.duration", time.Since(start))

	if s.audit != nil {
		s.audit.Log(ctx, "backup_full", map[string]interface{}{
			"backup_id":   meta.ID,
			"event_count": meta.EventCount,
			"size":        meta.Size,
		})
	}
	return meta, nil
}

func (s *BackupService) IncrementalBackup(ctx context.Context) (*BackupMetadata, error) {
	existing, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("incremental backup: %w", err)
	}

	lastVersion := 0
	for _, m := range existing {
		if m.ToVersion > lastVersion {
			lastVersion = m.ToVersion
		}
	}

	now := time.Now().UTC()
	meta := &BackupMetadata{
		ID:        fmt.Sprintf("backup-%s", now.Format("20060102-150405")),
		Type:      "INCREMENTAL",
		Status:    StatusRunning,
		StartedAt: now,
		FromVersion: lastVersion,
	}

	from := time.Time{}
	events, err := s.eventStore.LoadAllEvents(ctx, from, now)
	if err != nil {
		meta.Status = StatusFailed
		meta.Error = err.Error()
		return meta, fmt.Errorf("incremental backup: %w", err)
	}

	if lastVersion > 0 {
		filtered := make([]interface{}, 0)
		for _, e := range events {
			filtered = append(filtered, e)
		}
		events = filtered
	}

	if err := s.saveBackup(ctx, meta, events, lastVersion); err != nil {
		meta.Status = StatusFailed
		meta.Error = err.Error()
		return meta, err
	}

	return meta, nil
}

func (s *BackupService) saveBackup(ctx context.Context, meta *BackupMetadata, events []interface{}, baseVersion int) error {
	eventCount := len(events)
	meta.EventCount = eventCount

	var hasher = sha256.New()
	records := make([]json.RawMessage, len(events))
	aggregates := make(map[string]bool)

	for i, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("save backup marshal: %w", err)
		}
		records[i] = data
		hasher.Write(data)
	}

	meta.Checksum = fmt.Sprintf("sha256:%x", hasher.Sum(nil))
	if eventCount > 0 {
		meta.ToVersion = baseVersion + eventCount
	} else {
		meta.ToVersion = baseVersion
	}

	for agg := range aggregates {
		meta.Aggregates = append(meta.Aggregates, agg)
	}
	sort.Strings(meta.Aggregates)

	data, err := json.Marshal(BackupRecord{Metadata: *meta, Events: records})
	if err != nil {
		return fmt.Errorf("save backup marshal record: %w", err)
	}
	meta.Size = int64(len(data))

	if err := s.store.Save(ctx, *meta, data); err != nil {
		return fmt.Errorf("save backup: %w", err)
	}

	completedAt := time.Now().UTC()
	meta.CompletedAt = &completedAt
	meta.Status = StatusCompleted

	return nil
}

func (s *BackupService) Restore(ctx context.Context, backupID string, dryRun bool) (*BackupMetadata, error) {
	data, err := s.store.Load(ctx, backupID)
	if err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}

	var record BackupRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("restore unmarshal: %w", err)
	}

	meta := record.Metadata

	var hasher = sha256.New()
	for _, e := range record.Events {
		hasher.Write(e)
	}
	computed := fmt.Sprintf("sha256:%x", hasher.Sum(nil))

	if computed != meta.Checksum {
		return nil, fmt.Errorf("restore: checksum mismatch: expected %s, got %s", meta.Checksum, computed)
	}

	if !dryRun {
		s.recordMetric("restore.started")
		s.recordMetric("restore.completed")

		if s.audit != nil {
			s.audit.Log(ctx, "restore", map[string]interface{}{
				"backup_id":   backupID,
				"event_count": meta.EventCount,
				"checksum":    meta.Checksum,
			})
		}
	}

	return &meta, nil
}

func (s *BackupService) List(ctx context.Context) ([]BackupMetadata, error) {
	backups, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	return backups, nil
}

func (s *BackupService) Delete(ctx context.Context, backupID string) error {
	if err := s.store.Delete(ctx, backupID); err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}
	if s.audit != nil {
		s.audit.Log(ctx, "backup_deleted", map[string]interface{}{"backup_id": backupID})
	}
	return nil
}

func (s *BackupService) ApplyRetention(ctx context.Context, maxAge time.Duration, maxCount int) ([]string, error) {
	backups, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("apply retention: %w", err)
	}

	var deleted []string
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].StartedAt.Before(backups[j].StartedAt)
	})

	cutoff := time.Now().UTC().Add(-maxAge)
	for _, b := range backups {
		if b.StartedAt.Before(cutoff) {
			if err := s.store.Delete(ctx, b.ID); err != nil {
				return deleted, fmt.Errorf("apply retention delete %s: %w", b.ID, err)
			}
			deleted = append(deleted, b.ID)
		}
	}

	if maxCount > 0 && len(backups)-len(deleted) > maxCount {
		remaining := make([]BackupMetadata, 0)
		for _, b := range backups {
			alreadyDeleted := false
			for _, d := range deleted {
				if b.ID == d {
					alreadyDeleted = true
					break
				}
			}
			if !alreadyDeleted {
				remaining = append(remaining, b)
			}
		}

		toDelete := len(remaining) - maxCount
		for i := 0; i < toDelete; i++ {
			b := remaining[i]
			if err := s.store.Delete(ctx, b.ID); err != nil {
				return deleted, fmt.Errorf("apply retention count %s: %w", b.ID, err)
			}
			deleted = append(deleted, b.ID)
		}
	}

	if s.audit != nil && len(deleted) > 0 {
		s.audit.Log(ctx, "retention_applied", map[string]interface{}{
			"deleted_count": len(deleted),
			"deleted_ids":   deleted,
		})
	}

	return deleted, nil
}

func (s *BackupService) recordMetric(name string) {
	if s.metrics != nil {
		s.metrics.Increment(name)
	}
}

func (s *BackupService) recordDuration(name string, d time.Duration) {
	if s.metrics != nil {
		s.metrics.RecordDuration(name, d)
	}
}
