package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type EventRecord struct {
	ID            string
	AggregateID   string
	AggregateType string
	EventType     string
	Payload       json.RawMessage
	Version       int
	OccurredAt    time.Time
}

type SnapshotRecord struct {
	AggregateID   string
	AggregateType string
	State         json.RawMessage
	Version       int
	CreatedAt     time.Time
}

type (
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	beginFn    func(ctx context.Context) (pgx.Tx, error)
)

type EventStore struct {
	query    queryFn
	queryRow queryRowFn
	exec     execFn
	begin    beginFn
}

func NewEventStore(pool *pgxpool.Pool) *EventStore {
	return &EventStore{
		query:    pool.Query,
		queryRow: pool.QueryRow,
		exec:     pool.Exec,
		begin:    pool.Begin,
	}
}

func NewEventStoreWithFuncs(query queryFn, queryRow queryRowFn, exec execFn, begin beginFn) *EventStore {
	return &EventStore{query: query, queryRow: queryRow, exec: exec, begin: begin}
}

func (s *EventStore) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS events (
			id            TEXT PRIMARY KEY,
			aggregate_id TEXT NOT NULL,
			aggregate_type TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			payload       JSONB NOT NULL,
			version       INTEGER NOT NULL,
			occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_aggregate ON events(aggregate_id, version)`,
		`CREATE TABLE IF NOT EXISTS aggregate_snapshots (
			aggregate_id   TEXT NOT NULL,
			aggregate_type TEXT NOT NULL,
			state          JSONB NOT NULL,
			version        INTEGER NOT NULL,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (aggregate_id)
		)`,
	}

	for _, q := range queries {
		if _, err := s.exec(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	return nil
}

func (s *EventStore) Append(ctx context.Context, expectedVersion int, event EventRecord) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return fmt.Errorf("append: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.appendEvent(ctx, tx, expectedVersion, event); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("append: commit tx: %w", err)
	}

	return nil
}

func (s *EventStore) AppendWithTx(ctx context.Context, tx pgx.Tx, expectedVersion int, event EventRecord) error {
	return s.appendEvent(ctx, tx, expectedVersion, event)
}

func (s *EventStore) appendEvent(ctx context.Context, qr interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}, expectedVersion int, event EventRecord) error {
	var currentVersion int
	err := qr.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM events WHERE aggregate_id = $1`,
		event.AggregateID,
	).Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("append: check version: %w", err)
	}

	if expectedVersion >= 0 && currentVersion != expectedVersion {
		return fmt.Errorf("%w: expected version %d, got %d", common.ErrConcurrencyConflict, expectedVersion, currentVersion)
	}

	newVersion := currentVersion + 1
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("append: marshal payload: %w", err)
	}

	_, err = qr.Exec(ctx,
		`INSERT INTO events (id, aggregate_id, aggregate_type, event_type, payload, version, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		event.ID, event.AggregateID, event.AggregateType, event.EventType,
		payload, newVersion, event.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("append: insert event: %w", err)
	}

	return nil
}

type domainEvent interface {
	EventID() string
	EventType() string
}

func (s *EventStore) AppendDomainEvent(ctx context.Context, aggregateID, aggregateType string, expectedVersion int, event interface{}) error {
	evt, ok := event.(domainEvent)
	if !ok {
		return fmt.Errorf("append domain event: event does not implement domainEvent interface")
	}

	return s.Append(ctx, expectedVersion, EventRecord{
		ID:            evt.EventID(),
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     evt.EventType(),
		Payload:       mustMarshal(event),
		OccurredAt:    time.Now().UTC(),
	})
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func (s *EventStore) LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]EventRecord, error) {
	rows, err := s.query(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, payload, version, occurred_at
		 FROM events
		 WHERE aggregate_id = $1 AND version > $2
		 ORDER BY version ASC`,
		aggregateID, fromVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("load stream: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.AggregateType, &e.EventType,
			&e.Payload, &e.Version, &e.OccurredAt); err != nil {
			return nil, fmt.Errorf("load stream: scan: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load stream: rows: %w", err)
	}

	return events, nil
}

func (s *EventStore) Replay(ctx context.Context, aggregateID string) ([]interface{}, error) {
	records, err := s.LoadStream(ctx, aggregateID, 0)
	if err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}

	events := make([]interface{}, len(records))
	for i, r := range records {
		events[i] = r.Payload
	}

	return events, nil
}

func (s *EventStore) Snapshot(ctx context.Context, aggregateID, aggregateType string, state interface{}, version int) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("snapshot: marshal: %w", err)
	}

	_, err = s.exec(ctx,
		`INSERT INTO aggregate_snapshots (aggregate_id, aggregate_type, state, version, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (aggregate_id) DO UPDATE
		 SET state = $3, version = $4, created_at = $5`,
		aggregateID, aggregateType, stateJSON, version, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("snapshot: upsert: %w", err)
	}

	return nil
}

func (s *EventStore) LoadSnapshot(ctx context.Context, aggregateID string) (*SnapshotRecord, error) {
	var snap SnapshotRecord
	err := s.queryRow(ctx,
		`SELECT aggregate_id, aggregate_type, state, version, created_at
		 FROM aggregate_snapshots
		 WHERE aggregate_id = $1`,
		aggregateID,
	).Scan(&snap.AggregateID, &snap.AggregateType, &snap.State, &snap.Version, &snap.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found for aggregate %s: %w", aggregateID, err)
		}
		return nil, fmt.Errorf("load snapshot: %w", err)
	}

	return &snap, nil
}
