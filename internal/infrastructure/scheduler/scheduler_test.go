package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func newTestStore() *testStore {
	return &testStore{jobs: make(map[string]*Job)}
}

func (s *testStore) Save(ctx context.Context, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *job
	s.jobs[job.ID] = &cp
	return nil
}

func (s *testStore) FindByID(ctx context.Context, id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *job
	return &cp, nil
}

func (s *testStore) FindAll(ctx context.Context) ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		cp := *j
		result = append(result, &cp)
	}
	return result, nil
}

func (s *testStore) FindPending(ctx context.Context) ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []*Job
	for _, j := range s.jobs {
		if j.Status == StatusPending {
			cp := *j
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *testStore) Update(ctx context.Context, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *job
	s.jobs[job.ID] = &cp
	return nil
}

func (s *testStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return nil
}

type testLogger struct {
	entries []string
	mu      sync.Mutex
}

func (l *testLogger) Debug(msg string, kv ...interface{}) {}
func (l *testLogger) Info(msg string, kv ...interface{})  { l.record(msg) }
func (l *testLogger) Warn(msg string, kv ...interface{})  { l.record(msg) }
func (l *testLogger) Error(msg string, kv ...interface{}) { l.record(msg) }
func (l *testLogger) record(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, msg)
}

type testMetrics struct {
	counters  map[string]int64
	durations map[string]time.Duration
	mu        sync.Mutex
}

func newTestMetrics() *testMetrics {
	return &testMetrics{
		counters:  make(map[string]int64),
		durations: make(map[string]time.Duration),
	}
}

func (m *testMetrics) IncrementCounter(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}

func (m *testMetrics) RecordDuration(name string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations[name] = d
}

func TestScheduler_ScheduleOnce(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	executed := make(chan struct{}, 1)
	job := &Job{
		ID:           "job-1",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now().Add(50 * time.Millisecond),
		Func: func(ctx context.Context) error {
			executed <- struct{}{}
			return nil
		},
	}

	err := s.Schedule(job)
	assert.NoError(t, err)

	s.Start()
	defer s.Stop()

	select {
	case <-executed:
	case <-time.After(2 * time.Second):
		t.Fatal("job did not execute")
	}

	s.mu.Lock()
	j, ok := s.jobs["job-1"]
	s.mu.Unlock()
	assert.True(t, ok)
	assert.Equal(t, StatusCompleted, j.Status)
}

func TestScheduler_ScheduleRecurring(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	var mu sync.Mutex
	count := 0
	job := &Job{
		ID:           "job-recur",
		ScheduleType: ScheduleRecurring,
		Interval:     20 * time.Millisecond,
		Func: func(ctx context.Context) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		},
	}

	s.Schedule(job)
	s.Start()

	time.Sleep(300 * time.Millisecond)
	s.Stop()

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, count, 2)
}

func TestScheduler_ScheduleCron(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	executed := make(chan struct{}, 1)
	nextMin := time.Now().Add(2 * time.Minute)
	expr := fmt.Sprintf("%d %d %d %d *", nextMin.Minute(), nextMin.Hour(), nextMin.Day(), int(nextMin.Month()))

	job := &Job{
		ID:           "job-cron",
		ScheduleType: ScheduleCron,
		CronExpr:     expr,
		RunAt:        time.Now(),
		Func: func(ctx context.Context) error {
			select {
			case executed <- struct{}{}:
			default:
			}
			return nil
		},
	}

	s.Schedule(job)
	s.Start()
	defer s.Stop()

	select {
	case <-executed:
	case <-time.After(2*time.Minute + 5*time.Second):
		t.Fatal("cron job did not execute")
	}
}

func TestScheduler_Cancel(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	job := &Job{
		ID:           "job-cancel",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now().Add(10 * time.Second),
		Func:         func(ctx context.Context) error { return nil },
	}

	s.Schedule(job)
	err := s.Cancel("job-cancel")
	assert.NoError(t, err)

	j, ok := s.Get("job-cancel")
	assert.True(t, ok)
	assert.Equal(t, StatusCancelled, j.Status)
}

func TestScheduler_Cancel_NotFound(t *testing.T) {
	s := NewScheduler(nil)
	err := s.Cancel("nonexistent")
	assert.Error(t, err)
}

func TestScheduler_Retry(t *testing.T) {
	store := newTestStore()
	logger := &testLogger{}
	s := NewScheduler(store, WithLogger(logger))

	attempts := 0
	job := &Job{
		ID:           "job-retry",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now().Add(20 * time.Millisecond),
		MaxRetries:   2,
		Backoff:      func(attempt int) time.Duration { return 0 },
		Func: func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary failure")
			}
			return nil
		},
	}

	s.Schedule(job)
	s.Start()
	defer s.Stop()

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 3, attempts)

	j, _ := s.Get("job-retry")
	assert.Equal(t, StatusCompleted, j.Status)
}

func TestScheduler_RetryExhausted(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	job := &Job{
		ID:           "job-fail",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now().Add(20 * time.Millisecond),
		MaxRetries:   1,
		Backoff:      func(attempt int) time.Duration { return 0 },
		Func: func(ctx context.Context) error {
			return errors.New("permanent failure")
		},
	}

	s.Schedule(job)
	s.Start()
	defer s.Stop()

	time.Sleep(200 * time.Millisecond)

	j, _ := s.Get("job-fail")
	assert.Equal(t, StatusFailed, j.Status)
	assert.Contains(t, j.LastError, "permanent failure")
}

func TestScheduler_Timeout(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	job := &Job{
		ID:           "job-timeout",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now().Add(10 * time.Millisecond),
		Timeout:      30 * time.Millisecond,
		Func: func(ctx context.Context) error {
			select {
			case <-time.After(500 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	s.Schedule(job)
	s.Start()
	defer s.Stop()

	time.Sleep(200 * time.Millisecond)

	j, _ := s.Get("job-timeout")
	assert.Equal(t, StatusFailed, j.Status)
}

func TestScheduler_Recover(t *testing.T) {
	store := newTestStore()

	store.jobs["pending-1"] = &Job{
		ID:           "pending-1",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now(),
		Status:       StatusPending,
		Func: func(ctx context.Context) error { return nil },
	}

	s := NewScheduler(store)
	err := s.Recover(context.Background())
	assert.NoError(t, err)

	j, ok := s.Get("pending-1")
	assert.True(t, ok)
	assert.Equal(t, StatusPending, j.Status)
}

func TestScheduler_Schedule_Validation(t *testing.T) {
	s := NewScheduler(nil)

	t.Run("empty id", func(t *testing.T) {
		err := s.Schedule(&Job{ScheduleType: ScheduleOnce})
		assert.Error(t, err)
	})

	t.Run("nil func", func(t *testing.T) {
		err := s.Schedule(&Job{ID: "j", ScheduleType: ScheduleOnce})
		assert.Error(t, err)
	})

	t.Run("once without run_at", func(t *testing.T) {
		err := s.Schedule(&Job{ID: "j", ScheduleType: ScheduleOnce, Func: func(ctx context.Context) error { return nil }})
		assert.Error(t, err)
	})

	t.Run("recurring without interval", func(t *testing.T) {
		err := s.Schedule(&Job{ID: "j", ScheduleType: ScheduleRecurring, Func: func(ctx context.Context) error { return nil }})
		assert.Error(t, err)
	})

	t.Run("cron without expression", func(t *testing.T) {
		err := s.Schedule(&Job{ID: "j", ScheduleType: ScheduleCron, Func: func(ctx context.Context) error { return nil }})
		assert.Error(t, err)
	})
}

func TestScheduler_List(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	for i := 0; i < 5; i++ {
		s.Schedule(&Job{
			ID:           fmt.Sprintf("job-%d", i),
			ScheduleType: ScheduleOnce,
			RunAt:        time.Now().Add(time.Duration(i) * time.Second),
			Func:         func(ctx context.Context) error { return nil },
		})
	}

	list := s.List()
	assert.Len(t, list, 5)
}

func TestScheduler_Events(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	job := &Job{
		ID:           "job-evt",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now().Add(10 * time.Millisecond),
		Func:         func(ctx context.Context) error { return nil },
	}

	s.Schedule(job)
	s.Start()

	var events []SchedulerEvent
	timeout := time.After(time.Second)
loop:
	for {
		select {
		case evt := <-s.Events():
			events = append(events, evt)
			if len(events) >= 3 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}
	s.Stop()

	assertHasEvent(t, events, EventJobScheduled)
	assertHasEvent(t, events, EventJobStarted)
	assertHasEvent(t, events, EventJobCompleted)
}

func TestScheduler_Metrics(t *testing.T) {
	store := newTestStore()
	metrics := newTestMetrics()
	s := NewScheduler(store, WithMetrics(metrics))

	job := &Job{
		ID:           "job-metrics",
		ScheduleType: ScheduleOnce,
		RunAt:        time.Now().Add(10 * time.Millisecond),
		Func:         func(ctx context.Context) error { return nil },
	}

	s.Schedule(job)
	s.Start()
	defer s.Stop()

	time.Sleep(300 * time.Millisecond)

	assert.Greater(t, metrics.counters["scheduler.job.completed"], int64(0))
	d, ok := metrics.durations["scheduler.job.duration"]
	assert.True(t, ok)
	assert.Greater(t, d, time.Duration(0))
}

func TestCronParser_Valid(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		expr string
		want time.Time
	}{
		{"0 12 * * *", time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)},
		{"30 14 15 * *", time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)},
		{"0 0 1 1 *", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"*/5 * * * *", time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)},
		{"* * * * *", time.Date(2026, 1, 1, 12, 1, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			next, err := NextCronTime(tt.expr, now)
			assert.NoError(t, err)
			assert.True(t, next.Equal(tt.want), "expected %s, got %s for expr %s", tt.want, next, tt.expr)
		})
	}
}

func TestCronParser_Invalid(t *testing.T) {
	now := time.Now()

	tests := []string{
		"", "a", "* *", "* * * *",
		"60 * * * *", "* 24 * * *", "* * 32 * *", "* * * 13 *",
	}

	for _, expr := range tests {
		_, err := NextCronTime(expr, now)
		assert.Error(t, err, "expected error for %s", expr)
	}
}

func assertHasEvent(t *testing.T, events []SchedulerEvent, eventType SchedulerEventType) {
	t.Helper()
	for _, ev := range events {
		if ev.EventType == eventType {
			return
		}
	}
	t.Errorf("expected event %s", eventType)
}

func BenchmarkScheduler_Schedule(b *testing.B) {
	store := newTestStore()
	s := NewScheduler(store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Schedule(&Job{
			ID:           fmt.Sprintf("bench-%d", i),
			ScheduleType: ScheduleOnce,
			RunAt:        time.Now().Add(time.Hour),
			Func:         func(ctx context.Context) error { return nil },
		})
	}
}
