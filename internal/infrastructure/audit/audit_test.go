package audit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testEventStore struct {
	events []Envelope
}

func (s *testEventStore) LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]Envelope, error) {
	var result []Envelope
	for _, e := range s.events {
		if e.Meta.AggregateID == aggregateID && e.Meta.Version > fromVersion {
			result = append(result, e)
		}
	}
	return result, nil
}

func (s *testEventStore) LoadAllEvents(ctx context.Context, from, to time.Time) ([]Envelope, error) {
	return s.events, nil
}

type testSnapshotStore struct {
	snapshots map[string]json.RawMessage
	versions  map[string]int
}

func newTestSnapshotStore() *testSnapshotStore {
	return &testSnapshotStore{
		snapshots: make(map[string]json.RawMessage),
		versions:  make(map[string]int),
	}
}

func (s *testSnapshotStore) Save(ctx context.Context, aggregateID string, state json.RawMessage, version int) error {
	s.snapshots[aggregateID] = state
	s.versions[aggregateID] = version
	return nil
}

func (s *testSnapshotStore) Load(ctx context.Context, aggregateID string) (json.RawMessage, int, error) {
	return s.snapshots[aggregateID], s.versions[aggregateID], nil
}

func makeEvent(id, aggID, eventType string, version int, payload interface{}) Envelope {
	data, _ := json.Marshal(payload)
	return Envelope{
		Meta: EventMetadata{
			ID:            id,
			AggregateID:   aggID,
			AggregateType: "mission",
			EventType:     eventType,
			Version:       version,
			SchemaVersion: 1,
			OccurredAt:    time.Now(),
		},
		Payload: data,
	}
}

func TestUpcaster_RegisterAndUpcast(t *testing.T) {
	chain := NewUpcasterChain()

	chain.Register("MissionCreated", 1, func(raw json.RawMessage) (json.RawMessage, error) {
		var m map[string]interface{}
		json.Unmarshal(raw, &m)
		m["new_field"] = "added_in_v2"
		return json.Marshal(m)
	})

	original := json.RawMessage(`{"name": "test"}`)
	result, err := chain.Upcast("MissionCreated", 1, original)

	assert.NoError(t, err)
	assert.Contains(t, string(result), "added_in_v2")
	assert.Contains(t, string(result), "test")
}

func TestUpcaster_NoUpcasters(t *testing.T) {
	chain := NewUpcasterChain()
	original := json.RawMessage(`{"name": "test"}`)
	result, err := chain.Upcast("UnknownEvent", 5, original)
	assert.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestUpcaster_LatestVersion(t *testing.T) {
	chain := NewUpcasterChain()
	chain.Register("TestEvent", 1, func(raw json.RawMessage) (json.RawMessage, error) { return raw, nil })
	chain.Register("TestEvent", 2, func(raw json.RawMessage) (json.RawMessage, error) { return raw, nil })
	assert.Equal(t, 3, chain.LatestVersion("TestEvent"))
	assert.Equal(t, 1, chain.LatestVersion("Unknown"))
}

func TestCorrelator_Correlate(t *testing.T) {
	store := &testEventStore{
		events: []Envelope{
			makeEvent("e1", "m1", "TargetDiscovered", 1, map[string]interface{}{"type": "recon"}),
			makeEvent("e2", "m1", "PlanGenerated", 2, map[string]interface{}{}),
			makeEvent("e3", "m1", "ExploitExecuted", 3, map[string]interface{}{"type": "exploit"}),
			makeEvent("e4", "m1", "SessionCreated", 4, map[string]interface{}{}),
		},
	}

	c := NewEventCorrelator(store)
	result, err := c.Correlate(context.Background(), "m1", time.Time{}, time.Now())

	assert.NoError(t, err)
	assert.Equal(t, "m1", result.MissionID)
	assert.Equal(t, 4, result.TotalEvents)
	assert.NotNil(t, result.Phases["RECONNAISSANCE"])
	assert.NotNil(t, result.Phases["WEAPONIZATION"])
	assert.NotNil(t, result.Phases["EXPLOITATION"])
}

func TestCorrelator_BuildKillChainVisual(t *testing.T) {
	store := &testEventStore{
		events: []Envelope{
			makeEvent("e1", "m1", "TargetDiscovered", 1, map[string]interface{}{"type": "recon"}),
			makeEvent("e2", "m1", "ExploitExecuted", 3, map[string]interface{}{"type": "exploit"}),
		},
	}

	c := NewEventCorrelator(store)
	data, err := c.BuildKillChainVisual(context.Background(), "m1", time.Time{}, time.Now())

	assert.NoError(t, err)
	assert.Equal(t, "m1", data.MissionID)
	assert.Equal(t, 2, data.TotalEvents)
	assert.NotEmpty(t, data.Phases)
}

func TestReportGenerator_Generate(t *testing.T) {
	store := &testEventStore{
		events: []Envelope{
			makeEvent("e1", "m1", "TargetDiscovered", 1, map[string]interface{}{"description": "Host discovered"}),
			makeEvent("e2", "m1", "ExploitExecuted", 2, map[string]interface{}{"type": "exploit"}),
		},
	}
	snap := newTestSnapshotStore()

	c := NewEventCorrelator(store)
	replay := NewReplayEngine(store, snap, NewUpcasterChain())
	gen := NewReportGenerator(c, replay, NewUpcasterChain())

	report, err := gen.GenerateReport(context.Background(), "m1", time.Time{}, time.Now())

	assert.NoError(t, err)
	assert.Equal(t, 2, report.TotalEvents)
	assert.Len(t, report.Phases, 2)
}

func TestReportGenerator_ExportMarkdown(t *testing.T) {
	report := &AuditReport{
		MissionID:   "m1",
		TotalEvents: 2,
		Phases: []PhaseSection{
			{Name: "RECONNAISSANCE", EventCount: 1, Events: []EventSummary{
				{ID: "e1", Type: "TargetDiscovered", Timestamp: time.Now(), Summary: "Host found"},
			}},
		},
		Findings: []FindingSummary{{Title: "Test", Severity: "High", CVSS: 7.5}},
	}

	gen := &ReportGenerator{}
	md := gen.ExportMarkdown(report)

	assert.Contains(t, md, "# Ophidian Mission Audit Report")
	assert.Contains(t, md, "m1")
	assert.Contains(t, md, "RECONNAISSANCE")
	assert.Contains(t, md, "Test")
}

func TestReportGenerator_ExportBurpXML(t *testing.T) {
	report := &AuditReport{
		Findings: []FindingSummary{{Title: "SQL Injection", Severity: "High"}},
	}
	gen := &ReportGenerator{}
	xml := gen.ExportBurpXML(report)
	assert.Contains(t, xml, "SQL Injection")
	assert.Contains(t, xml, "issues")
}

func TestReportGenerator_ExportJSON(t *testing.T) {
	report := &AuditReport{
		MissionID:   "m1",
		TotalEvents: 1,
	}
	gen := &ReportGenerator{}
	j := gen.ExportJSON(report)
	assert.Contains(t, j, "m1")
	assert.True(t, json.Valid([]byte(j)))
}

func TestReplayEngine_Rebuild(t *testing.T) {
	store := &testEventStore{
		events: []Envelope{
			makeEvent("e1", "agg-1", "MissionCreated", 1, map[string]string{"name": "test"}),
			makeEvent("e2", "agg-1", "MissionStarted", 2, map[string]string{"status": "active"}),
		},
	}
	snap := newTestSnapshotStore()

	engine := NewReplayEngine(store, snap, NewUpcasterChain())
	engine.RegisterProjection(&testProjection{name: "mission-view"})

	err := engine.Rebuild(context.Background(), "agg-1")
	assert.NoError(t, err)

	loaded, ver, _ := snap.Load(context.Background(), "agg-1")
	assert.NotNil(t, loaded)
	assert.Equal(t, 2, ver)
}

type testProjection struct {
	name    string
	applied int
}

func (p *testProjection) ProjectionName() string             { return p.name }
func (p *testProjection) Apply(ctx context.Context, e Envelope) error { p.applied++; return nil }
func (p *testProjection) State() json.RawMessage             { return json.RawMessage(`{"count":2}`) }
func (p *testProjection) Version() int                       { return 2 }

func TestWrapEnvelope(t *testing.T) {
	meta := EventMetadata{
		ID:            "evt-1",
		AggregateID:   "agg-1",
		AggregateType: "mission",
		EventType:     "MissionCreated",
		Version:       1,
		OccurredAt:    time.Now(),
	}
	env, err := Wrap(meta, map[string]string{"key": "value"})
	assert.NoError(t, err)
	assert.Equal(t, "evt-1", env.Meta.ID)

	var payload map[string]string
	err = env.UnmarshalPayload(&payload)
	assert.NoError(t, err)
	assert.Equal(t, "value", payload["key"])
}
