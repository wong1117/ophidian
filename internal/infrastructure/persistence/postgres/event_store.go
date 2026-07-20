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
	ID             string
	AggregateID    string
	AggregateType  string
	EventType      string
	Payload        json.RawMessage
	Version        int
	OccurredAt     time.Time
	CorrelationID  string
	CausationID    string
	Metadata       map[string]interface{}
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
			id              TEXT PRIMARY KEY,
			aggregate_id   TEXT NOT NULL,
			aggregate_type TEXT NOT NULL,
			event_type     TEXT NOT NULL,
			payload        JSONB NOT NULL,
			version        INTEGER NOT NULL,
			occurred_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			correlation_id TEXT,
			causation_id   TEXT,
			metadata       JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_aggregate ON events(aggregate_id, version)`,
		`CREATE INDEX IF NOT EXISTS idx_events_occurred_at ON events(occurred_at)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(aggregate_type, event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_correlation ON events(correlation_id) WHERE correlation_id IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS aggregate_snapshots (
			aggregate_id   TEXT NOT NULL,
			aggregate_type TEXT NOT NULL,
			state          JSONB NOT NULL,
			version        INTEGER NOT NULL,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (aggregate_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_type ON aggregate_snapshots(aggregate_type)`,
	}

	for _, q := range queries {
		if _, err := s.exec(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	return nil
}

func checkCtx(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("operation cancelled: %w", ctx.Err())
	default:
		return nil
	}
}

func (s *EventStore) Append(ctx context.Context, expectedVersion int, event EventRecord) error {
	if err := checkCtx(ctx); err != nil {
		return fmt.Errorf("append: %w", err)
	}

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

func (s *EventStore) AppendBatch(ctx context.Context, events []EventRecord) error {
	if err := checkCtx(ctx); err != nil {
		return fmt.Errorf("append batch: %w", err)
	}

	tx, err := s.begin(ctx)
	if err != nil {
		return fmt.Errorf("append batch: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, event := range events {
		if err := s.appendEvent(ctx, tx, -1, event); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("append batch: commit tx: %w", err)
	}

	return nil
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

	var metadataJSON []byte
	if event.Metadata != nil {
		metadataJSON, _ = json.Marshal(event.Metadata)
	}

	_, err = qr.Exec(ctx,
		`INSERT INTO events (id, aggregate_id, aggregate_type, event_type, payload, version, occurred_at, correlation_id, causation_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO NOTHING`,
		event.ID, event.AggregateID, event.AggregateType, event.EventType,
		payload, newVersion, event.OccurredAt,
		nullIfEmpty(event.CorrelationID),
		nullIfEmpty(event.CausationID),
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("append: insert event: %w", err)
	}

	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("append domain event: marshal: %w", err)
	}

	return s.Append(ctx, expectedVersion, EventRecord{
		ID:            evt.EventID(),
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     evt.EventType(),
		Payload:       payload,
		OccurredAt:    time.Now().UTC(),
	})
}

func (s *EventStore) LoadStream(ctx context.Context, aggregateID string, fromVersion int) ([]EventRecord, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, fmt.Errorf("load stream: %w", err)
	}

	rows, err := s.query(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, payload, version, occurred_at,
		 correlation_id, causation_id, metadata
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
		var correlationID, causationID *string
		var metadataJSON []byte
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.AggregateType, &e.EventType,
			&e.Payload, &e.Version, &e.OccurredAt, &correlationID, &causationID, &metadataJSON); err != nil {
			return nil, fmt.Errorf("load stream: scan: %w", err)
		}
		if correlationID != nil {
			e.CorrelationID = *correlationID
		}
		if causationID != nil {
			e.CausationID = *causationID
		}
		if len(metadataJSON) > 0 && string(metadataJSON) != "null" {
			json.Unmarshal(metadataJSON, &e.Metadata)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load stream: rows: %w", err)
	}

	return events, nil
}

func (s *EventStore) Replay(ctx context.Context, aggregateID string) ([]interface{}, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}

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

func (s *EventStore) ReplayFromSnapshot(ctx context.Context, aggregateID string) ([]interface{}, int, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, 0, fmt.Errorf("replay from snapshot: %w", err)
	}
	snap, err := s.LoadSnapshot(ctx, aggregateID)
	if err != nil {
		return nil, 0, fmt.Errorf("replay from snapshot: %w", err)
	}
	records, err := s.LoadStream(ctx, aggregateID, snap.Version)
	if err != nil {
		return nil, 0, fmt.Errorf("replay from snapshot: %w", err)
	}
	events := make([]interface{}, len(records))
	for i, r := range records {
		events[i] = r.Payload
	}
	return events, snap.Version, nil
}

func (s *EventStore) LoadAllEvents(ctx context.Context, from, to time.Time) ([]EventRecord, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, fmt.Errorf("load all events: %w", err)
	}

	rows, err := s.query(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, payload, version, occurred_at,
		 correlation_id, causation_id, metadata
		 FROM events
		 WHERE occurred_at >= $1 AND occurred_at <= $2
		 ORDER BY occurred_at DESC
		 LIMIT 10000`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("load all events: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		var correlationID, causationID *string
		var metadataJSON []byte
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.AggregateType, &e.EventType,
			&e.Payload, &e.Version, &e.OccurredAt, &correlationID, &causationID, &metadataJSON); err != nil {
			return nil, fmt.Errorf("load all events scan: %w", err)
		}
		if correlationID != nil {
			e.CorrelationID = *correlationID
		}
		if causationID != nil {
			e.CausationID = *causationID
		}
		if len(metadataJSON) > 0 && string(metadataJSON) != "null" {
			json.Unmarshal(metadataJSON, &e.Metadata)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load all events rows: %w", err)
	}

	return events, nil
}

func (s *EventStore) Snapshot(ctx context.Context, aggregateID, aggregateType string, state interface{}, version int) error {
	if err := checkCtx(ctx); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

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
	if err := checkCtx(ctx); err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}

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
