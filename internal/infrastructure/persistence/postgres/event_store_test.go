package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/stretchr/testify/assert"
)

type testRow struct {
	scanFn func(dest ...any) error
}

func (r testRow) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return nil
}

type testTx struct {
	queryRowRet testRow
	execErr     error
	commitErr   error
	rollbackErr error
	execFn      func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (tx *testTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.queryRowRet
}

func (tx *testTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.execFn != nil {
		return tx.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, tx.execErr
}

func (tx *testTx) Commit(ctx context.Context) error  { return tx.commitErr }
func (tx *testTx) Rollback(ctx context.Context) error { return tx.rollbackErr }

func (tx *testTx) Begin(ctx context.Context) (pgx.Tx, error)                          { return nil, nil }
func (tx *testTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *testTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (tx *testTx) LargeObjects() pgx.LargeObjects                                   { return pgx.LargeObjects{} }
func (tx *testTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *testTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) { return nil, nil }
func (tx *testTx) Conn() *pgx.Conn                                                       { return nil }

func sampleEvent(aggregateID string, idx int) EventRecord {
	return EventRecord{
		ID:             fmt.Sprintf("evt-%s-%d", aggregateID, idx),
		AggregateID:    aggregateID,
		AggregateType:  "mission",
		EventType:      "MissionCreated",
		Payload:        json.RawMessage(`{"action":"create"}`),
		OccurredAt:     time.Now().UTC(),
		CorrelationID:  "corr-123",
		CausationID:    "cause-456",
		Metadata:       map[string]interface{}{"env": "test"},
	}
}

func sampleEvents(aggregateID string, count int) []EventRecord {
	events := make([]EventRecord, count)
	for i := 0; i < count; i++ {
		events[i] = EventRecord{
			ID:            fmt.Sprintf("evt-%s-%d", aggregateID, i),
			AggregateID:   aggregateID,
			AggregateType: "mission",
			EventType:     fmt.Sprintf("Event%d", i),
			Payload:       json.RawMessage(fmt.Sprintf(`{"seq":%d}`, i)),
			OccurredAt:    time.Now().UTC(),
		}
	}
	return events
}

func TestEventStore_AppendWithTx_Success(t *testing.T) {
	ctx := context.Background()
	store := &EventStore{}

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			},
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			assert.Contains(t, sql, "ON CONFLICT (id) DO NOTHING")
			return pgconn.CommandTag{}, nil
		},
	}

	event := sampleEvent("agg-1", 0)
	err := store.AppendWithTx(ctx, tx, 0, event)

	assert.NoError(t, err)
}

func TestEventStore_AppendWithTx_ConcurrencyConflict(t *testing.T) {
	ctx := context.Background()
	store := &EventStore{}

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 3
				return nil
			},
		},
	}

	event := sampleEvent("agg-1", 0)
	err := store.AppendWithTx(ctx, tx, 0, event)

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrConcurrencyConflict)
	assert.Contains(t, err.Error(), "expected version 0, got 3")
}

func TestEventStore_AppendWithTx_VersionIncrement(t *testing.T) {
	ctx := context.Background()
	store := &EventStore{}

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 2
				return nil
			},
		},
	}

	event := sampleEvent("agg-2", 0)
	err := store.AppendWithTx(ctx, tx, 2, event)

	assert.NoError(t, err)
}

func TestEventStore_AppendWithTx_NoVersionCheck(t *testing.T) {
	ctx := context.Background()
	store := &EventStore{}

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 5
				return nil
			},
		},
	}

	event := sampleEvent("agg-3", 0)
	err := store.AppendWithTx(ctx, tx, -1, event)

	assert.NoError(t, err)
}

func TestEventStore_AppendWithTx_QueryRowError(t *testing.T) {
	ctx := context.Background()
	store := &EventStore{}

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				return errors.New("db error")
			},
		},
	}

	event := sampleEvent("agg-1", 0)
	err := store.AppendWithTx(ctx, tx, 0, event)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check version")
}

func TestEventStore_AppendWithTx_InsertError(t *testing.T) {
	ctx := context.Background()
	store := &EventStore{}

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			},
		},
		execErr: errors.New("insert error"),
	}

	event := sampleEvent("agg-1", 0)
	err := store.AppendWithTx(ctx, tx, 0, event)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert event")
}

func TestEventStore_Append_Success_FullFlow(t *testing.T) {
	ctx := context.Background()

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			},
		},
	}

	store := NewEventStoreWithFuncs(
		nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	)

	event := sampleEvent("agg-1", 0)
	err := store.Append(ctx, 0, event)

	assert.NoError(t, err)
}

func TestEventStore_Append_BeginError(t *testing.T) {
	ctx := context.Background()

	store := NewEventStoreWithFuncs(
		nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) {
			return nil, errors.New("connection refused")
		},
	)

	event := sampleEvent("agg-1", 0)
	err := store.Append(ctx, 0, event)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin tx")
}

func TestEventStore_Append_CommitError(t *testing.T) {
	ctx := context.Background()

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			},
		},
		commitErr: errors.New("commit failed"),
	}

	store := NewEventStoreWithFuncs(
		nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	)

	event := sampleEvent("agg-1", 0)
	err := store.Append(ctx, 0, event)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "commit tx")
}

func TestEventStore_Append_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := NewEventStoreWithFuncs(nil, nil, nil, nil)
	err := store.Append(ctx, 0, sampleEvent("agg-1", 0))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestEventStore_LoadStream_WithFuncs(t *testing.T) {
	ctx := context.Background()

	store := NewEventStoreWithFuncs(
		func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query failed")
		},
		nil, nil, nil,
	)

	events, err := store.LoadStream(ctx, "agg-1", 0)

	assert.Nil(t, events)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load stream")
}

func TestEventStore_LoadStream_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := NewEventStoreWithFuncs(nil, nil, nil, nil)
	_, err := store.LoadStream(ctx, "agg-1", 0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestEventStore_Snapshot_WithFuncs(t *testing.T) {
	ctx := context.Background()

	store := NewEventStoreWithFuncs(
		nil, nil,
		func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
		nil,
	)

	state := map[string]string{"name": "test"}
	err := store.Snapshot(ctx, "agg-1", "mission", state, 3)

	assert.NoError(t, err)
}

func TestEventStore_Snapshot_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := NewEventStoreWithFuncs(nil, nil, nil, nil)
	err := store.Snapshot(ctx, "agg-1", "mission", nil, 0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestEventStore_LoadSnapshot_WithFuncs(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		store := NewEventStoreWithFuncs(
			nil,
			func(ctx context.Context, sql string, args ...any) pgx.Row {
				return testRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*string) = "agg-1"
						*dest[1].(*string) = "mission"
						*dest[2].(*json.RawMessage) = json.RawMessage(`{"name":"test"}`)
						*dest[3].(*int) = 3
						*dest[4].(*time.Time) = time.Now().UTC()
						return nil
					},
				}
			},
			nil, nil,
		)

		snap, err := store.LoadSnapshot(ctx, "agg-1")

		assert.NoError(t, err)
		assert.NotNil(t, snap)
		assert.Equal(t, "agg-1", snap.AggregateID)
		assert.Equal(t, 3, snap.Version)
	})

	t.Run("not found", func(t *testing.T) {
		store := NewEventStoreWithFuncs(
			nil,
			func(ctx context.Context, sql string, args ...any) pgx.Row {
				return testRow{scanFn: func(dest ...any) error { return errors.New("no rows") }}
			},
			nil, nil,
		)

		snap, err := store.LoadSnapshot(ctx, "agg-1")

		assert.Nil(t, snap)
		assert.Error(t, err)
	})
}

func TestEventStore_AppendBatch(t *testing.T) {
	ctx := context.Background()

	version := 0
	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = version
				version++
				return nil
			},
		},
	}

	store := NewEventStoreWithFuncs(nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	)

	events := sampleEvents("agg-batch", 10)
	err := store.AppendBatch(ctx, events)

	assert.NoError(t, err)
	assert.Equal(t, 10, version)
}

func TestEventStore_ReplayFromSnapshot(t *testing.T) {
	ctx := context.Background()

	store := NewEventStoreWithFuncs(
		func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return newMockRows(nil), nil
		},
		func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*string) = "agg-1"
					*dest[1].(*string) = "mission"
					*dest[2].(*json.RawMessage) = json.RawMessage(`{"name":"state"}`)
					*dest[3].(*int) = 5
					*dest[4].(*time.Time) = time.Now().UTC()
					return nil
				},
			}
		},
		nil, nil,
	)

	events, baseVersion, err := store.ReplayFromSnapshot(ctx, "agg-1")

	assert.NoError(t, err)
	assert.Equal(t, 5, baseVersion)
	assert.NotNil(t, events)
	assert.Empty(t, events)
}

type testDomainEvent struct {
	evtID   string
	evtType string
}

func (e testDomainEvent) EventID() string   { return e.evtID }
func (e testDomainEvent) EventType() string { return e.evtType }

func TestEventStore_AppendDomainEvent_WithFuncs(t *testing.T) {
	ctx := context.Background()

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			},
		},
	}

	store := NewEventStoreWithFuncs(nil, nil, nil, func(ctx context.Context) (pgx.Tx, error) { return tx, nil })

	t.Run("success", func(t *testing.T) {
		evt := testDomainEvent{evtID: "evt-1", evtType: "TestEvent"}
		err := store.AppendDomainEvent(ctx, "agg-1", "mission", 0, evt)
		assert.NoError(t, err)
	})

	t.Run("non_domain_event", func(t *testing.T) {
		err := store.AppendDomainEvent(ctx, "agg-1", "mission", 0, "not an event")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not implement domainEvent")
	})
}

func TestEventStore_Concurrency(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	version := 0
	success := 0
	failures := 0
	k := 30

	var wg sync.WaitGroup
	var muRes sync.Mutex

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				mu.Lock()
				v := version
				mu.Unlock()
				*dest[0].(*int) = v
				return nil
			},
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			mu.Lock()
			version++
			mu.Unlock()
			return pgconn.CommandTag{}, nil
		},
	}

	store := NewEventStoreWithFuncs(nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	)

	for i := 0; i < k; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			event := sampleEvent("agg-concurrent", idx)
			err := store.Append(ctx, -1, event)

			muRes.Lock()
			if err != nil {
				failures++
			} else {
				success++
			}
			muRes.Unlock()
		}(i)
	}

	wg.Wait()

	assert.Equal(t, k, success+failures)
	t.Logf("concurrent appends: %d total, %d successes, %d failures", k, success, failures)
}

func TestEventStore_Concurrency_WithVersionConflicts(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	version := 0
	success := 0
	failures := 0
	k := 50

	var wg sync.WaitGroup
	var muRes sync.Mutex

	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				mu.Lock()
				v := version
				mu.Unlock()
				*dest[0].(*int) = v
				return nil
			},
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			mu.Lock()
			version++
			mu.Unlock()
			return pgconn.CommandTag{}, nil
		},
	}

	store := NewEventStoreWithFuncs(nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	)

	for i := 0; i < k; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			event := sampleEvent("agg-concurrent-vc", idx)
			err := store.Append(ctx, 0, event)

			muRes.Lock()
			if err != nil {
				failures++
			} else {
				success++
			}
			muRes.Unlock()
		}(i)
	}

	wg.Wait()

	assert.Equal(t, k, success+failures)
	t.Logf("concurrent appends with version check: %d total, %d successes, %d failures", k, success, failures)
	assert.Greater(t, failures, 0, "expected some version conflicts")
}

func BenchmarkEventStore_Append(b *testing.B) {
	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			},
		},
	}

	store := NewEventStoreWithFuncs(nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	)

	event := sampleEvent("bench-agg", 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Append(ctx, -1, event)
		event.ID = fmt.Sprintf("evt-bench-%d", i)
	}
}

func BenchmarkEventStore_AppendBatch(b *testing.B) {
	tx := &testTx{
		queryRowRet: testRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			},
		},
	}

	store := NewEventStoreWithFuncs(nil, nil, nil,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	)

	events := sampleEvents("bench-batch", 100)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.AppendBatch(ctx, events)
		for j := range events {
			events[j].ID = fmt.Sprintf("evt-bench-batch-%d-%d", i, j)
		}
	}
}

func BenchmarkEventStore_LoadStream(b *testing.B) {
	store := NewEventStoreWithFuncs(
		func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return newMockRows(nil), nil
		},
		nil, nil, nil,
	)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.LoadStream(ctx, "bench-load", 0)
	}
}
