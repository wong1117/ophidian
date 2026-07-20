package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type EventStore interface {
	LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]Envelope, error)
	LoadAllEvents(ctx context.Context, from, to time.Time) ([]Envelope, error)
}

type SnapshotStore interface {
	Save(ctx context.Context, aggregateID string, state json.RawMessage, version int) error
	Load(ctx context.Context, aggregateID string) (json.RawMessage, int, error)
}

type ReplayEngine struct {
	eventStore    EventStore
	snapshotStore SnapshotStore
	upcaster      *UpcasterChain
	projections   map[string]Projection
}

type Projection interface {
	ProjectionName() string
	Apply(ctx context.Context, event Envelope) error
	State() json.RawMessage
	Version() int
}

func NewReplayEngine(eventStore EventStore, snapshotStore SnapshotStore, upcaster *UpcasterChain) *ReplayEngine {
	return &ReplayEngine{
		eventStore:    eventStore,
		snapshotStore: snapshotStore,
		upcaster:      upcaster,
		projections:   make(map[string]Projection),
	}
}

func (r *ReplayEngine) RegisterProjection(p Projection) {
	r.projections[p.ProjectionName()] = p
}

func (r *ReplayEngine) Rebuild(ctx context.Context, aggregateID string) error {
	snapshotData, snapshotVersion, err := r.snapshotStore.Load(ctx, aggregateID)
	fromVersion := 0
	if err == nil && snapshotData != nil {
		fromVersion = snapshotVersion
		_ = snapshotData
	}

	events, err := r.eventStore.LoadStream(ctx, aggregateID, fromVersion)
	if err != nil {
		return fmt.Errorf("replay rebuild: %w", err)
	}

	for _, event := range events {
		if err := r.applyEvent(ctx, event); err != nil {
			return fmt.Errorf("replay apply %s: %w", event.Meta.ID, err)
		}
	}

	if len(events) > 0 {
		lastVersion := events[len(events)-1].Meta.Version
		for _, p := range r.projections {
			state := p.State()
			if err := r.snapshotStore.Save(ctx, aggregateID, state, lastVersion); err != nil {
				return fmt.Errorf("replay snapshot: %w", err)
			}
		}
	}

	return nil
}

func (r *ReplayEngine) RebuildAll(ctx context.Context, from, to time.Time) error {
	events, err := r.eventStore.LoadAllEvents(ctx, from, to)
	if err != nil {
		return fmt.Errorf("replay rebuild all: %w", err)
	}

	for _, event := range events {
		if err := r.applyEvent(ctx, event); err != nil {
			return fmt.Errorf("replay apply: %w", err)
		}
	}

	return nil
}

func (r *ReplayEngine) applyEvent(ctx context.Context, event Envelope) error {
	if r.upcaster != nil {
		upcasted, err := r.upcaster.Upcast(event.Meta.EventType, event.Meta.SchemaVersion, event.Payload)
		if err != nil {
			return err
		}
		event.Payload = upcasted
	}

	for _, p := range r.projections {
		if err := p.Apply(ctx, event); err != nil {
			return fmt.Errorf("projection %s apply: %w", p.ProjectionName(), err)
		}
	}

	return nil
}
