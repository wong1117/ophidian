package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/queue"
	"github.com/stretchr/testify/assert"
)

type testQueue struct {
	mu       sync.Mutex
	jobs     map[string]*queue.Job
	pending  []string
	dequeued map[string]bool
	acked    map[string]bool
	nacked   map[string]int
}

func newTestQueue() *testQueue {
	return &testQueue{
		jobs:     make(map[string]*queue.Job),
		dequeued: make(map[string]bool),
		acked:    make(map[string]bool),
		nacked:   make(map[string]int),
	}
}

func (q *testQueue) Dequeue(ctx interface{}) (*queue.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.pending) == 0 {
		return nil, nil
	}
	id := q.pending[0]
	q.pending = q.pending[1:]
	q.dequeued[id] = true
	job := q.jobs[id]
	if job == nil {
		return nil, nil
	}
	cp := *job
	return &cp, nil
}

func (q *testQueue) Ack(ctx interface{}, jobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.acked[jobID] = true
	return nil
}

func (q *testQueue) Nack(ctx interface{}, jobID string, err error) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.nacked[jobID]++
	q.pending = append(q.pending, jobID)
	return nil
}

func (q *testQueue) addJob(job *queue.Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs[job.ID] = job
	q.pending = append(q.pending, job.ID)
}

type testLogger struct {
	entries []string
	mu      sync.Mutex
}

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

func TestWorkerPool_RegisterWorker(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q)

	p.RegisterWorker("w1", []string{"handler-a", "handler-b"})

	workers := p.Workers()
	assert.Len(t, workers, 1)
	assert.Equal(t, "w1", workers[0].ID)
	assert.Contains(t, workers[0].Capabilities, "handler-a")
}

func TestWorkerPool_DispatchJob(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithHeartbeatTimeout(time.Hour), WithPollInterval(10*time.Millisecond))
	p.RegisterHandler("process", func(ctx context.Context, job *queue.Job) error {
		return nil
	})

	p.RegisterWorker("w1", []string{"process"})

	q.addJob(&queue.Job{ID: "job-1", Handler: "process"})

	p.Start()
	time.Sleep(100 * time.Millisecond)
	p.Stop()

	assert.True(t, q.acked["job-1"])
}

func TestWorkerPool_DispatchJob_NoHandler(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithPollInterval(10*time.Millisecond))

	p.RegisterWorker("w1", []string{"unknown"})
	q.addJob(&queue.Job{ID: "job-x", Handler: "unknown"})

	p.Start()
	time.Sleep(100 * time.Millisecond)
	p.Stop()

	assert.False(t, q.acked["job-x"])
}

func TestWorkerPool_RetryOnFailure(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithPollInterval(10*time.Millisecond))

	attempts := 0
	p.RegisterHandler("retry", func(ctx context.Context, job *queue.Job) error {
		attempts++
		if attempts < 3 {
			return errors.New("fail")
		}
		return nil
	})

	p.RegisterWorker("w1", []string{"retry"})

	q.addJob(&queue.Job{ID: "job-r", Handler: "retry"})

	p.Start()
	time.Sleep(300 * time.Millisecond)
	p.Stop()

	assert.GreaterOrEqual(t, attempts, 2)
	assert.True(t, q.acked["job-r"])
}

func TestWorkerPool_Heartbeat(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithHeartbeatTimeout(50*time.Millisecond))

	p.RegisterWorker("w1", []string{"handler"})

	p.Heartbeat("w1")

	w := p.Workers()[0]
	assert.WithinDuration(t, time.Now(), w.LastSeen, time.Second)
}

func TestWorkerPool_DeregisterWorker(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q)

	p.RegisterWorker("w1", []string{"handler"})
	assert.Len(t, p.Workers(), 1)

	p.DeregisterWorker("w1")
	assert.Empty(t, p.Workers())
}

func TestWorkerPool_Concurrency(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithPollInterval(10*time.Millisecond), WithMaxConcurrency(5))

	p.RegisterHandler("handler", func(ctx context.Context, job *queue.Job) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	})

	for i := 0; i < 5; i++ {
		p.RegisterWorker(fmt.Sprintf("w%d", i), []string{"handler"})
	}

	for i := 0; i < 20; i++ {
		q.addJob(&queue.Job{ID: fmt.Sprintf("j%d", i), Handler: "handler"})
	}

	p.Start()
	time.Sleep(500 * time.Millisecond)
	p.Stop()

	t.Logf("acked: %d", len(q.acked))
	assert.GreaterOrEqual(t, len(q.acked), 10)
}

func TestWorkerPool_Events(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithPollInterval(10*time.Millisecond))
	p.RegisterHandler("handler", func(ctx context.Context, job *queue.Job) error { return nil })

	p.RegisterWorker("w1", []string{"handler"})
	q.addJob(&queue.Job{ID: "job-evt", Handler: "handler"})

	p.Start()
	time.Sleep(200 * time.Millisecond)
	p.Stop()

	var events []WorkerEvent
	for {
		select {
		case ev := <-p.Events():
			events = append(events, ev)
		default:
			goto check
		}
	}
check:
	assertHasWorkerEvent(t, events, EventWorkerRegistered)
	assertHasWorkerEvent(t, events, EventJobCompleted)
}

func TestWorkerPool_Metrics(t *testing.T) {
	q := newTestQueue()
	m := newTestMetrics()
	p := NewWorkerPool(q, WithMetrics(m), WithPollInterval(10*time.Millisecond))
	p.RegisterHandler("handler", func(ctx context.Context, job *queue.Job) error { return nil })

	p.RegisterWorker("w1", []string{"handler"})
	q.addJob(&queue.Job{ID: "job-m", Handler: "handler"})

	p.Start()
	time.Sleep(200 * time.Millisecond)
	p.Stop()

	assert.Greater(t, m.counters["worker.job.completed"], int64(0))
	assert.Greater(t, m.counters["worker.registered.total"], int64(0))
}

func TestWorkerPool_Drain(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q)

	p.RegisterWorker("w1", []string{"handler"})
	p.Drain()

	for _, w := range p.Workers() {
		assert.Equal(t, WorkerStatusDraining, w.Status)
	}
}

func TestWorkerPool_CapabilityMatching(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithPollInterval(10*time.Millisecond))

	executed := make(map[string]bool)
	var mu sync.Mutex
	p.RegisterHandler("handler-a", func(ctx context.Context, job *queue.Job) error {
		mu.Lock()
		executed[job.ID] = true
		mu.Unlock()
		return nil
	})
	p.RegisterHandler("handler-b", func(ctx context.Context, job *queue.Job) error {
		return nil
	})

	p.RegisterWorker("w-a", []string{"handler-a"})
	p.RegisterWorker("w-b", []string{"handler-b"})

	q.addJob(&queue.Job{ID: "ja", Handler: "handler-a"})
	q.addJob(&queue.Job{ID: "jb", Handler: "handler-b"})

	p.Start()
	time.Sleep(200 * time.Millisecond)
	p.Stop()

	assert.True(t, executed["ja"])
}

func TestWorkerPool_HealthCheck(t *testing.T) {
	q := newTestQueue()
	p := NewWorkerPool(q, WithHeartbeatTimeout(10*time.Millisecond), WithHeartbeatInterval(20*time.Millisecond))

	p.RegisterWorker("w-expired", []string{"handler"})

	time.Sleep(50 * time.Millisecond)

	p.checkHealth()

	var found bool
	for _, w := range p.Workers() {
		if w.ID == "w-expired" && w.Status == WorkerStatusOffline {
			found = true
		}
	}
	assert.True(t, found, "worker should be marked offline after heartbeat timeout")
}

func assertHasWorkerEvent(t *testing.T, events []WorkerEvent, eventType WorkerEventType) {
	t.Helper()
	for _, ev := range events {
		if ev.EventType == eventType {
			return
		}
	}
	t.Errorf("expected event %s", eventType)
}

func BenchmarkWorkerPool_Dispatch(b *testing.B) {
	q := newTestQueue()
	p := NewWorkerPool(q)
	p.RegisterHandler("bench", func(ctx context.Context, job *queue.Job) error { return nil })
	p.RegisterWorker("w1", []string{"bench"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.addJob(&queue.Job{ID: fmt.Sprintf("b-%d", i), Handler: "bench"})
	}
}
