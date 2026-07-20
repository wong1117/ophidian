package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testEventStore struct {
	events []interface{}
}

func (s *testEventStore) LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]interface{}, error) {
	return s.events, nil
}

func (s *testEventStore) LoadAllEvents(ctx context.Context, from, to time.Time) ([]interface{}, error) {
	return s.events, nil
}

type testBackupStore struct {
	mu     sync.Mutex
	data   map[string][]byte
	metas  []BackupMetadata
}

func newTestBackupStore() *testBackupStore {
	return &testBackupStore{data: make(map[string][]byte)}
}

func (s *testBackupStore) Save(ctx context.Context, meta BackupMetadata, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[meta.ID] = data
	s.metas = append(s.metas, meta)
	return nil
}

func (s *testBackupStore) Load(ctx context.Context, backupID string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.data[backupID]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return d, nil
}

func (s *testBackupStore) List(ctx context.Context) ([]BackupMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]BackupMetadata, len(s.metas))
	copy(result, s.metas)
	return result, nil
}

func (s *testBackupStore) Delete(ctx context.Context, backupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, backupID)
	for i, m := range s.metas {
		if m.ID == backupID {
			s.metas = append(s.metas[:i], s.metas[i+1:]...)
			break
		}
	}
	return nil
}

type testAudit struct {
	mu      sync.Mutex
	actions []string
}

func (a *testAudit) Log(ctx context.Context, action string, details map[string]interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, action)
}

type testMetrics struct {
	mu        sync.Mutex
	counters  map[string]int64
	durations map[string]time.Duration
}

func newTestMetrics() *testMetrics {
	return &testMetrics{
		counters:  make(map[string]int64),
		durations: make(map[string]time.Duration),
	}
}

func (m *testMetrics) Increment(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name]++
}

func (m *testMetrics) RecordDuration(name string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations[name] = d
}

func TestBackupService_FullBackup(t *testing.T) {
	store := &testEventStore{
		events: []interface{}{
			map[string]string{"type": "MissionCreated", "id": "1"},
			map[string]string{"type": "MissionStarted", "id": "2"},
		},
	}
	backupStore := newTestBackupStore()
	audit := &testAudit{}
	metrics := newTestMetrics()

	svc := NewBackupService(store, backupStore).WithAudit(audit).WithMetrics(metrics)

	meta, err := svc.FullBackup(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "FULL", meta.Type)
	assert.Equal(t, BackupStatus("COMPLETED"), meta.Status)
	assert.Equal(t, 2, meta.EventCount)
	assert.NotEmpty(t, meta.Checksum)
	assert.Greater(t, meta.Size, int64(0))
	assert.Len(t, audit.actions, 1)
	assert.Equal(t, int64(1), metrics.counters["backup.started"])
	assert.Equal(t, int64(1), metrics.counters["backup.completed"])
}

func TestBackupService_IncrementalBackup(t *testing.T) {
	store := &testEventStore{
		events: []interface{}{map[string]string{"type": "Event1"}},
	}
	backupStore := newTestBackupStore()

	svc := NewBackupService(store, backupStore)

	meta1, _ := svc.FullBackup(context.Background())
	meta2, err := svc.IncrementalBackup(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "INCREMENTAL", meta2.Type)
	assert.Equal(t, meta1.ToVersion, meta2.FromVersion)
}

func TestBackupService_Restore(t *testing.T) {
	store := &testEventStore{
		events: []interface{}{map[string]string{"type": "Event1"}},
	}
	backupStore := newTestBackupStore()

	svc := NewBackupService(store, backupStore)
	meta, _ := svc.FullBackup(context.Background())

	restored, err := svc.Restore(context.Background(), meta.ID, false)
	assert.NoError(t, err)
	assert.Equal(t, meta.Checksum, restored.Checksum)
	assert.Equal(t, meta.EventCount, restored.EventCount)
}

func TestBackupService_Restore_ChecksumMismatch(t *testing.T) {
	backupStore := newTestBackupStore()
	fakeData, _ := json.Marshal(BackupRecord{
		Metadata: BackupMetadata{Checksum: "sha256:fake"},
		Events:   []json.RawMessage{json.RawMessage(`{"type":"Event1"}`)},
	})
	backupStore.data["fake-id"] = fakeData

	svc := NewBackupService(nil, backupStore)
	_, err := svc.Restore(context.Background(), "fake-id", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestBackupService_DryRunRestore(t *testing.T) {
	store := &testEventStore{
		events: []interface{}{map[string]string{"type": "Event1"}},
	}
	backupStore := newTestBackupStore()

	svc := NewBackupService(store, backupStore)
	meta, _ := svc.FullBackup(context.Background())

	restored, err := svc.Restore(context.Background(), meta.ID, true)
	assert.NoError(t, err)
	assert.NotNil(t, restored)
}

func TestBackupService_List(t *testing.T) {
	store := &testEventStore{events: []interface{}{map[string]string{"type": "E1"}}}
	backupStore := newTestBackupStore()
	svc := NewBackupService(store, backupStore)

	svc.FullBackup(context.Background())
	svc.FullBackup(context.Background())

	list, err := svc.List(context.Background())
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestBackupService_Delete(t *testing.T) {
	store := &testEventStore{events: []interface{}{map[string]string{"type": "E1"}}}
	backupStore := newTestBackupStore()
	svc := NewBackupService(store, backupStore)

	meta, _ := svc.FullBackup(context.Background())
	err := svc.Delete(context.Background(), meta.ID)
	assert.NoError(t, err)

	list, _ := svc.List(context.Background())
	assert.Empty(t, list)
}

func TestBackupService_Retention(t *testing.T) {
	store := &testEventStore{events: []interface{}{map[string]string{"type": "E1"}}}
	backupStore := newTestBackupStore()
	audit := &testAudit{}

	for i := 0; i < 5; i++ {
		backupStore.metas = append(backupStore.metas, BackupMetadata{
			ID:        fmt.Sprintf("old-%d", i),
			StartedAt: time.Now().Add(-48 * time.Hour),
			Type:      "FULL",
		})
	}

	svc := NewBackupService(store, backupStore).WithAudit(audit)
	deleted, err := svc.ApplyRetention(context.Background(), 24*time.Hour, 2)

	assert.NoError(t, err)
	assert.Len(t, deleted, 5)
	assert.NotEmpty(t, audit.actions)
}

func TestBackupService_Retention_CountLimit(t *testing.T) {
	store := &testEventStore{events: []interface{}{map[string]string{"type": "E1"}}}
	backupStore := newTestBackupStore()
	svc := NewBackupService(store, backupStore)

	svc.FullBackup(context.Background())
	svc.FullBackup(context.Background())
	svc.FullBackup(context.Background())

	deleted, err := svc.ApplyRetention(context.Background(), 365*24*time.Hour, 1)

	assert.NoError(t, err)
	assert.Len(t, deleted, 2)

	list, _ := svc.List(context.Background())
	assert.Len(t, list, 1)
}

func TestBackupService_Restore_NotFound(t *testing.T) {
	backupStore := newTestBackupStore()
	svc := NewBackupService(nil, backupStore)

	_, err := svc.Restore(context.Background(), "nonexistent", false)
	assert.Error(t, err)
}
