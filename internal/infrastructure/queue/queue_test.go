package queue

import (
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

func (s *testStore) Save(ctx interface{}, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *job
	s.jobs[job.ID] = &cp
	return nil
}

func (s *testStore) FindByID(ctx interface{}, id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *j
	return &cp, nil
}

func (s *testStore) ListPending(ctx interface{}) ([]*Job, error) {
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

func (s *testStore) Update(ctx interface{}, job *Job) error  { return s.Save(ctx, job) }
func (s *testStore) Delete(ctx interface{}, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return nil
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

func TestQueue_EnqueueDequeueAck(t *testing.T) {
	q := NewPriorityQueue(nil)

	job := &Job{ID: "job-1", Handler: "test", Priority: 5}
	err := q.Enqueue(job)
	assert.NoError(t, err)
	assert.Equal(t, 1, q.Size())
	assert.Equal(t, 1, q.Pending())

	dequeued, err := q.Dequeue(nil)
	assert.NoError(t, err)
	assert.NotNil(t, dequeued)
	assert.Equal(t, "job-1", dequeued.ID)
	assert.Equal(t, StatusRunning, dequeued.Status)
	assert.Equal(t, 0, q.Size())
	assert.Equal(t, 1, q.InFlight())

	err = q.Ack(nil, "job-1")
	assert.NoError(t, err)
	assert.Equal(t, 0, q.InFlight())
}

func TestQueue_Priority(t *testing.T) {
	q := NewPriorityQueue(nil)

	q.Enqueue(&Job{ID: "low", Handler: "test", Priority: 1})
	q.Enqueue(&Job{ID: "high", Handler: "test", Priority: 10})
	q.Enqueue(&Job{ID: "mid", Handler: "test", Priority: 5})

	j1, _ := q.Dequeue(nil)
	j2, _ := q.Dequeue(nil)
	j3, _ := q.Dequeue(nil)

	assert.Equal(t, "high", j1.ID)
	assert.Equal(t, "mid", j2.ID)
	assert.Equal(t, "low", j3.ID)
}

func TestQueue_DelayedJob(t *testing.T) {
	q := NewPriorityQueue(nil)

	err := q.Enqueue(&Job{ID: "delayed", Handler: "test", Delay: 100 * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, 0, q.Size())
	assert.Equal(t, 1, q.Delayed())

	j, _ := q.Dequeue(nil)
	assert.Nil(t, j)

	time.Sleep(150 * time.Millisecond)

	j, err = q.Dequeue(nil)
	assert.NoError(t, err)
	assert.NotNil(t, j)
	assert.Equal(t, "delayed", j.ID)
}

func TestQueue_Retry(t *testing.T) {
	q := NewPriorityQueue(nil)

	q.Enqueue(&Job{ID: "retry", Handler: "test", MaxRetries: 2})

	_, err := q.Dequeue(nil)
	assert.NoError(t, err)

	q.Nack(nil, "retry", errors.New("transient error"))

	assert.Equal(t, 1, q.Size())
	j2, _ := q.Dequeue(nil)
	assert.NotNil(t, j2)
	assert.Equal(t, "retry", j2.ID)
}

func TestQueue_DeadLetter(t *testing.T) {
	q := NewPriorityQueue(nil)

	q.Enqueue(&Job{ID: "dlq", Handler: "test", MaxRetries: 1})

	_, _ = q.Dequeue(nil)
	q.Nack(nil, "dlq", errors.New("fail 1"))

	_, _ = q.Dequeue(nil)
	q.Nack(nil, "dlq", errors.New("fail 2"))

	assert.Equal(t, 0, q.Size())
	assert.Equal(t, 1, q.DeadLettered())

	deadJobs, _ := q.DrainDeadLetter(nil)
	assert.Len(t, deadJobs, 1)
	assert.Equal(t, "dlq", deadJobs[0].ID)
	assert.Equal(t, 0, q.DeadLettered())
}

func TestQueue_RequeueDeadLetter(t *testing.T) {
	q := NewPriorityQueue(nil)

	q.Enqueue(&Job{ID: "requeue", Handler: "test", MaxRetries: 0})
	_, _ = q.Dequeue(nil)
	q.Nack(nil, "requeue", errors.New("fail"))
	assert.Equal(t, 1, q.DeadLettered())

	err := q.RequeueDeadLetter(nil, "requeue")
	assert.NoError(t, err)
	assert.Equal(t, 0, q.DeadLettered())
	assert.Equal(t, 1, q.Size())
}

func TestQueue_AckMissing(t *testing.T) {
	q := NewPriorityQueue(nil)
	err := q.Ack(nil, "no-such-job")
	assert.NoError(t, err)
}

func TestQueue_NackMissing(t *testing.T) {
	q := NewPriorityQueue(nil)
	err := q.Nack(nil, "no-such-job", errors.New("err"))
	assert.NoError(t, err)
}

func TestQueue_Stats(t *testing.T) {
	q := NewPriorityQueue(nil)

	q.Enqueue(&Job{ID: "a", Handler: "test", Priority: 1})
	q.Enqueue(&Job{ID: "b", Handler: "test", Priority: 2})
	q.Enqueue(&Job{ID: "c", Handler: "test", Delay: time.Hour})

	stats := q.Stats()
	assert.Equal(t, 2, stats.Pending)
	assert.Equal(t, 0, stats.Inflight)
	assert.Equal(t, 1, stats.Delayed)
	assert.Equal(t, 0, stats.DeadLettered)
}

func TestQueue_Events(t *testing.T) {
	q := NewPriorityQueue(nil)

	q.Enqueue(&Job{ID: "evt", Handler: "test"})
	j, _ := q.Dequeue(nil)
	q.Ack(nil, j.ID)

	var events []QueueEvent
	for {
		select {
		case evt := <-q.Events():
			events = append(events, evt)
		default:
			goto done
		}
	}
done:
	assertHasQueueEvent(t, events, EventJobEnqueued)
	assertHasQueueEvent(t, events, EventJobDequeued)
	assertHasQueueEvent(t, events, EventJobAcknowledged)
}

func TestQueue_Metrics(t *testing.T) {
	m := newTestMetrics()
	q := NewPriorityQueue(nil, WithQueueMetrics(m))

	q.Enqueue(&Job{ID: "m", Handler: "test"})
	j, _ := q.Dequeue(nil)
	q.Ack(nil, j.ID)

	assert.Equal(t, int64(1), m.counters["queue.enqueued.total"])
	assert.Equal(t, int64(1), m.counters["queue.dequeued.total"])
	assert.Equal(t, int64(1), m.counters["queue.completed.total"])
	assert.NotZero(t, m.durations["queue.processing.duration"])
}

func TestQueue_Concurrency(t *testing.T) {
	q := NewPriorityQueue(nil)
	k := 50

	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			q.Enqueue(&Job{ID: fmt.Sprintf("c-%d", idx), Handler: "test", Priority: idx % 10})
		}(i)
	}
	wg.Wait()

	assert.Equal(t, k, q.Size())

	for i := 0; i < k; i++ {
		j, err := q.Dequeue(nil)
		assert.NoError(t, err)
		assert.NotNil(t, j)
		q.Ack(nil, j.ID)
	}
}

func TestQueue_Logger(t *testing.T) {
	l := &testLogger{}
	q := NewPriorityQueue(nil, WithQueueLogger(l))

	q.Enqueue(&Job{ID: "log", Handler: "test", Delay: time.Millisecond})
	assert.NotEmpty(t, l.entries)
}

func assertHasQueueEvent(t *testing.T, events []QueueEvent, eventType QueueEventType) {
	t.Helper()
	for _, ev := range events {
		if ev.EventType == eventType {
			return
		}
	}
	t.Errorf("expected event %s", eventType)
}

func BenchmarkQueue_Enqueue(b *testing.B) {
	q := NewPriorityQueue(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(&Job{ID: fmt.Sprintf("b-%d", i), Handler: "bench", Priority: i % 100})
	}
}

func BenchmarkQueue_DequeueAck(b *testing.B) {
	q := NewPriorityQueue(nil)
	for i := 0; i < 1000; i++ {
		q.Enqueue(&Job{ID: fmt.Sprintf("bd-%d", i), Handler: "bench"})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j, _ := q.Dequeue(nil)
		if j != nil {
			q.Ack(nil, j.ID)
			q.Enqueue(&Job{ID: fmt.Sprintf("bd-replenish-%d", i), Handler: "bench"})
		}
	}
}
