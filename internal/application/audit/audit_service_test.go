package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/persistence/postgres"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
	"github.com/stretchr/testify/assert"
)

type testEventStore struct {
	events []postgres.EventRecord
}

func (s *testEventStore) LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]postgres.EventRecord, error) {
	return s.events, nil
}

func (s *testEventStore) LoadAllEvents(ctx context.Context, from, to time.Time) ([]postgres.EventRecord, error) {
	return s.events, nil
}

func sampleEvent(id, aggType, evtType string, payload string, ago time.Duration) postgres.EventRecord {
	return postgres.EventRecord{
		ID:            id,
		AggregateID:   "agg-" + id,
		AggregateType: aggType,
		EventType:     evtType,
		Payload:       json.RawMessage(payload),
		CorrelationID: "corr-" + id,
		OccurredAt:    time.Now().UTC().Add(-ago),
	}
}

func TestAuditService_Query(t *testing.T) {
	store := &testEventStore{
		events: []postgres.EventRecord{
			sampleEvent("1", "mission", "MissionCreated", `{"tenant_id":"t1","user_id":"u1"}`, time.Hour),
			sampleEvent("2", "mission", "MissionStarted", `{"tenant_id":"t1"}`, 30*time.Minute),
			sampleEvent("3", "attack", "PlanGenerated", `{"tenant_id":"t2"}`, 10*time.Minute),
		},
	}

	svc := NewAuditService(store)
	resp, err := svc.Query(context.Background(), dto.AuditFilter{PerPage: 10})

	assert.NoError(t, err)
	assert.Equal(t, 3, resp.Total)
	assert.Len(t, resp.Entries, 3)
}

func TestAuditService_Query_FilterByType(t *testing.T) {
	store := &testEventStore{
		events: []postgres.EventRecord{
			sampleEvent("1", "mission", "MissionCreated", `{}`, time.Hour),
			sampleEvent("2", "attack", "PlanGenerated", `{}`, 30*time.Minute),
		},
	}

	svc := NewAuditService(store)
	resp, err := svc.Query(context.Background(), dto.AuditFilter{
		AggregateType: "mission",
		PerPage:       10,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

func TestAuditService_Query_FilterByTenant(t *testing.T) {
	store := &testEventStore{
		events: []postgres.EventRecord{
			sampleEvent("1", "mission", "MissionCreated", `{"tenant_id":"acme"}`, time.Hour),
			sampleEvent("2", "mission", "MissionStarted", `{"tenant_id":"other"}`, 30*time.Minute),
		},
	}

	svc := NewAuditService(store)
	resp, err := svc.Query(context.Background(), dto.AuditFilter{
		TenantID: "acme",
		PerPage:  10,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

func TestAuditService_Query_Search(t *testing.T) {
	store := &testEventStore{
		events: []postgres.EventRecord{
			sampleEvent("evt-abc", "mission", "MissionCreated", `{"key":"value"}`, time.Hour),
			sampleEvent("evt-xyz", "attack", "PlanGenerated", `{"key":"other"}`, 30*time.Minute),
		},
	}

	svc := NewAuditService(store)
	resp, err := svc.Query(context.Background(), dto.AuditFilter{Search: "abc", PerPage: 10})

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

func TestAuditService_Query_Pagination(t *testing.T) {
	store := &testEventStore{}
	for i := 0; i < 25; i++ {
		store.events = append(store.events, sampleEvent(
			"evt", "mission", "Event", "{}", time.Duration(i)*time.Minute,
		))
	}

	svc := NewAuditService(store)
	resp, err := svc.Query(context.Background(), dto.AuditFilter{Page: 1, PerPage: 10})

	assert.NoError(t, err)
	assert.Equal(t, 25, resp.Total)
	assert.Len(t, resp.Entries, 10)
	assert.Equal(t, 3, resp.TotalPages)
}

func TestAuditService_ExportJSON(t *testing.T) {
	store := &testEventStore{
		events: []postgres.EventRecord{
			sampleEvent("1", "mission", "MissionCreated", `{"key":"value"}`, time.Hour),
		},
	}

	svc := NewAuditService(store)
	data, err := svc.ExportJSON(context.Background(), dto.AuditFilter{})

	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.True(t, strings.HasPrefix(string(data), "["))
}

func TestAuditService_ExportCSV(t *testing.T) {
	store := &testEventStore{
		events: []postgres.EventRecord{
			sampleEvent("1", "mission", "MissionCreated", `{"key":"value"}`, time.Hour),
		},
	}

	svc := NewAuditService(store)
	data, err := svc.ExportCSV(context.Background(), dto.AuditFilter{})

	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "MissionCreated")
}

func TestAuditService_Metrics(t *testing.T) {
	store := &testEventStore{
		events: []postgres.EventRecord{
			sampleEvent("1", "mission", "MissionCreated", `{}`, time.Hour),
			sampleEvent("2", "mission", "MissionStarted", `{}`, 30*time.Minute),
			sampleEvent("3", "attack", "PlanGenerated", `{}`, 10*time.Minute),
		},
	}

	svc := NewAuditService(store)
	metrics, err := svc.Metrics(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, int64(3), metrics.TotalEvents)
	assert.Equal(t, int64(2), metrics.EventsByType["MissionCreated"]+metrics.EventsByType["MissionStarted"])
	assert.Greater(t, metrics.RecentActivity, int64(0))
}

func TestAuditService_EmptyResults(t *testing.T) {
	store := &testEventStore{}
	svc := NewAuditService(store)

	resp, err := svc.Query(context.Background(), dto.AuditFilter{PerPage: 10})
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Total)

	jsonData, _ := svc.ExportJSON(context.Background(), dto.AuditFilter{})
	assert.Contains(t, string(jsonData), "[]")

	csvData, _ := svc.ExportCSV(context.Background(), dto.AuditFilter{})
	assert.NotEmpty(t, csvData)

	metrics, _ := svc.Metrics(context.Background())
	assert.Equal(t, int64(0), metrics.TotalEvents)
}
