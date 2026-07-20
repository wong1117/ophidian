package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/persistence/postgres"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type EventStream interface {
	LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]postgres.EventRecord, error)
}

type FullEventStream interface {
	LoadAllEvents(ctx context.Context, from, to time.Time) ([]postgres.EventRecord, error)
}

type AuditService struct {
	eventStore  FullEventStream
}

func NewAuditService(store FullEventStream) *AuditService {
	return &AuditService{eventStore: store}
}

func (s *AuditService) Query(ctx context.Context, filter dto.AuditFilter) (*dto.AuditResponse, error) {
	now := time.Now().UTC()
	if filter.From == nil {
		t := now.Add(-24 * time.Hour)
		filter.From = &t
	}
	if filter.To == nil {
		filter.To = &now
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 || filter.PerPage > 100 {
		filter.PerPage = 20
	}

	events, err := s.eventStore.LoadAllEvents(ctx, *filter.From, *filter.To)
	if err != nil {
		return nil, fmt.Errorf("audit query: %w", err)
	}

	filtered := s.applyFilters(events, filter)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].OccurredAt.After(filtered[j].OccurredAt)
	})

	total := len(filtered)
	start := (filter.Page - 1) * filter.PerPage
	if start > total {
		start = total
	}
	end := start + filter.PerPage
	if end > total {
		end = total
	}

	entries := make([]dto.AuditEntry, 0, len(filtered[start:end]))
	for _, e := range filtered[start:end] {
		entry := mapRecord(e)
		entries = append(entries, entry)
	}

	totalPages := total / filter.PerPage
	if total%filter.PerPage > 0 {
		totalPages++
	}

	return &dto.AuditResponse{
		Entries:    entries,
		Total:      total,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (s *AuditService) applyFilters(events []postgres.EventRecord, filter dto.AuditFilter) []postgres.EventRecord {
	var result []postgres.EventRecord
	for _, e := range events {
		if filter.AggregateType != "" && !strings.EqualFold(e.AggregateType, filter.AggregateType) {
			continue
		}
		if filter.EventType != "" && !strings.EqualFold(e.EventType, filter.EventType) {
			continue
		}
		if filter.TenantID != "" && !containsPayloadField(e.Payload, "tenant_id", filter.TenantID) {
			continue
		}
		if filter.UserID != "" && !containsPayloadField(e.Payload, "user_id", filter.UserID) {
			continue
		}
		if filter.Search != "" {
			if !matchesSearch(e, filter.Search) {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

func containsPayloadField(payload json.RawMessage, key, value string) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(fmt.Sprintf("%v", v)), strings.ToLower(value))
}

func matchesSearch(event postgres.EventRecord, query string) bool {
	lower := strings.ToLower(query)
	if strings.Contains(strings.ToLower(event.ID), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(event.EventType), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(event.AggregateType), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(string(event.Payload)), lower) {
		return true
	}
	return false
}

func mapRecord(e postgres.EventRecord) dto.AuditEntry {
	var payload map[string]interface{}
	json.Unmarshal(e.Payload, &payload)
	return dto.AuditEntry{
		ID:            e.ID,
		AggregateID:   e.AggregateID,
		AggregateType: e.AggregateType,
		EventType:     e.EventType,
		Payload:       payload,
		CorrelationID: e.CorrelationID,
		OccurredAt:    e.OccurredAt,
	}
}

func (s *AuditService) ExportJSON(ctx context.Context, filter dto.AuditFilter) ([]byte, error) {
	resp, err := s.Query(ctx, dto.AuditFilter{
		AggregateType: filter.AggregateType,
		EventType:     filter.EventType,
		TenantID:      filter.TenantID,
		UserID:        filter.UserID,
		Search:        filter.Search,
		From:          filter.From,
		To:            filter.To,
		Page:          1,
		PerPage:       10000,
	})
	if err != nil {
		return nil, fmt.Errorf("export json: %w", err)
	}

	data, err := json.MarshalIndent(resp.Entries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("export json marshal: %w", err)
	}
	return data, nil
}

func (s *AuditService) ExportCSV(ctx context.Context, filter dto.AuditFilter) ([]byte, error) {
	resp, err := s.Query(ctx, dto.AuditFilter{
		AggregateType: filter.AggregateType,
		EventType:     filter.EventType,
		TenantID:      filter.TenantID,
		UserID:        filter.UserID,
		Search:        filter.Search,
		From:          filter.From,
		To:            filter.To,
		Page:          1,
		PerPage:       10000,
	})
	if err != nil {
		return nil, fmt.Errorf("export csv: %w", err)
	}

	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Write([]string{"ID", "AggregateID", "AggregateType", "EventType", "Payload", "CorrelationID", "OccurredAt"})
	for _, e := range resp.Entries {
		payloadJSON, _ := json.Marshal(e.Payload)
		w.Write([]string{
			e.ID, e.AggregateID, e.AggregateType, e.EventType,
			string(payloadJSON), e.CorrelationID, e.OccurredAt.Format(time.RFC3339),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("export csv write: %w", err)
	}
	return []byte(buf.String()), nil
}

func (s *AuditService) Metrics(ctx context.Context) (*dto.AuditMetrics, error) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	events, err := s.eventStore.LoadAllEvents(ctx, from, now)
	if err != nil {
		return nil, fmt.Errorf("audit metrics: %w", err)
	}

	m := &dto.AuditMetrics{
		TotalEvents:        int64(len(events)),
		EventsByType:       make(map[string]int64),
		EventsByAggregate:  make(map[string]int64),
	}

	recentFrom := now.Add(-24 * time.Hour)
	for _, e := range events {
		m.EventsByType[e.EventType]++
		m.EventsByAggregate[e.AggregateType]++
		if e.OccurredAt.After(recentFrom) {
			m.RecentActivity++
		}
	}

	return m, nil
}
