package queue

import (
	"container/heap"
	"sync"
	"time"
)

type JobStatus string

const (
	StatusPending    JobStatus = "PENDING"
	StatusRunning    JobStatus = "RUNNING"
	StatusCompleted  JobStatus = "COMPLETED"
	StatusFailed     JobStatus = "FAILED"
	StatusDeadLettered JobStatus = "DEAD_LETTERED"
	StatusDelayed    JobStatus = "DELAYED"
)

type Job struct {
	ID          string
	Payload     interface{}
	Handler     string
	Priority    int
	Delay       time.Duration
	MaxRetries  int
	RetryCount  int
	Status      JobStatus
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	EnqueuedAt  time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	VisibleAt   time.Time
}

func (j *Job) Clone() *Job {
	cp := *j
	return &cp
}

type QueueEventType string

const (
	EventJobEnqueued      QueueEventType = "JOB_ENQUEUED"
	EventJobDequeued      QueueEventType = "JOB_DEQUEUED"
	EventJobAcknowledged  QueueEventType = "JOB_ACKNOWLEDGED"
	EventJobFailed        QueueEventType = "JOB_FAILED"
	EventJobRetrying      QueueEventType = "JOB_RETRYING"
	EventJobDeadLettered  QueueEventType = "JOB_DEAD_LETTERED"
	EventJobDelayed       QueueEventType = "JOB_DELAYED"
)

type QueueEvent struct {
	JobID     string
	Handler   string
	EventType QueueEventType
	Timestamp time.Time
	Error     string
}

type JobStore interface {
	Save(ctx interface{}, job *Job) error
	FindByID(ctx interface{}, id string) (*Job, error)
	ListPending(ctx interface{}) ([]*Job, error)
	Update(ctx interface{}, job *Job) error
	Delete(ctx interface{}, id string) error
}

type Logger interface {
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
}

type Metrics interface {
	IncrementCounter(name string, value int64)
	RecordDuration(name string, d time.Duration)
}

type PriorityQueue struct {
	mu       sync.Mutex
	heap     *jobHeap
	pending  map[string]*Job
	delayed  []*Job
	inflight map[string]*Job
	dead     map[string]*Job
	store    JobStore
	logger   Logger
	metrics  Metrics
	eventCh  chan QueueEvent
}

type jobHeap struct {
	jobs []*Job
}

func (h *jobHeap) Len() int           { return len(h.jobs) }
func (h *jobHeap) Less(i, j int) bool { return h.jobs[i].Priority > h.jobs[j].Priority }
func (h *jobHeap) Swap(i, j int)      { h.jobs[i], h.jobs[j] = h.jobs[j], h.jobs[i] }
func (h *jobHeap) Push(x interface{}) { h.jobs = append(h.jobs, x.(*Job)) }
func (h *jobHeap) Pop() interface{} {
	old := h.jobs
	n := len(old)
	x := old[n-1]
	h.jobs = old[0 : n-1]
	return x
}

type QueueOption func(*PriorityQueue)

func WithQueueLogger(logger Logger) QueueOption {
	return func(q *PriorityQueue) { q.logger = logger }
}

func WithQueueMetrics(metrics Metrics) QueueOption {
	return func(q *PriorityQueue) { q.metrics = metrics }
}

func NewPriorityQueue(store JobStore, opts ...QueueOption) *PriorityQueue {
	q := &PriorityQueue{
		heap:     &jobHeap{jobs: make([]*Job, 0)},
		pending:  make(map[string]*Job),
		delayed:  make([]*Job, 0),
		inflight: make(map[string]*Job),
		dead:     make(map[string]*Job),
		store:    store,
		eventCh:  make(chan QueueEvent, 100),
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

func (q *PriorityQueue) Events() <-chan QueueEvent {
	return q.eventCh
}

func (q *PriorityQueue) Enqueue(job *Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	if job.Delay > 0 {
		job.Status = StatusDelayed
		job.VisibleAt = now.Add(job.Delay)
		job.EnqueuedAt = now
		q.delayed = append(q.delayed, job.Clone())
		q.emit(QueueEvent{JobID: job.ID, Handler: job.Handler, EventType: EventJobDelayed, Timestamp: now})
		q.logInfo("job delayed", "id", job.ID, "delay_ms", job.Delay.Milliseconds())
	} else {
		job.Status = StatusPending
		job.VisibleAt = now
		job.EnqueuedAt = now
		cp := job.Clone()
		q.pending[cp.ID] = cp
		heap.Push(q.heap, cp)
		q.emit(QueueEvent{JobID: job.ID, Handler: job.Handler, EventType: EventJobEnqueued, Timestamp: now})
	}

	if q.store != nil {
		if err := q.store.Save(nil, job); err != nil {
			return err
		}
	}

	q.metricCounter("queue.enqueued.total", 1)
	return nil
}

func (q *PriorityQueue) Dequeue(ctx interface{}) (*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.promoteDelayed()

	if q.heap.Len() == 0 {
		return nil, nil
	}

	item := heap.Pop(q.heap).(*Job)
	delete(q.pending, item.ID)

	now := time.Now()
	item.Status = StatusRunning
	item.StartedAt = &now
	item.UpdatedAt = now

	q.inflight[item.ID] = item.Clone()
	q.emit(QueueEvent{JobID: item.ID, Handler: item.Handler, EventType: EventJobDequeued, Timestamp: now})

	if q.store != nil {
		_ = q.store.Update(nil, item)
	}

	q.metricCounter("queue.dequeued.total", 1)
	return item.Clone(), nil
}

func (q *PriorityQueue) Ack(ctx interface{}, jobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, ok := q.inflight[jobID]
	if !ok {
		return nil
	}

	now := time.Now()
	job.Status = StatusCompleted
	job.CompletedAt = &now
	job.UpdatedAt = now
	delete(q.inflight, jobID)

	q.emit(QueueEvent{JobID: jobID, Handler: job.Handler, EventType: EventJobAcknowledged, Timestamp: now})
	if q.store != nil {
		_ = q.store.Delete(nil, jobID)
	}

	q.metricDuration("queue.processing.duration", now.Sub(*job.StartedAt))
	q.metricCounter("queue.completed.total", 1)
	return nil
}

func (q *PriorityQueue) Nack(ctx interface{}, jobID string, err error) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, ok := q.inflight[jobID]
	if !ok {
		return nil
	}

	now := time.Now()
	job.RetryCount++
	job.LastError = err.Error()
	job.UpdatedAt = now

	if job.RetryCount > job.MaxRetries {
		job.Status = StatusDeadLettered
		delete(q.inflight, jobID)
		q.dead[jobID] = job.Clone()
		q.emit(QueueEvent{JobID: jobID, Handler: job.Handler, EventType: EventJobDeadLettered, Timestamp: now, Error: err.Error()})
		q.logWarn("job dead lettered", "id", jobID, "retries", job.RetryCount, "error", err.Error())
		q.metricCounter("queue.dead_lettered.total", 1)
	} else {
		job.Status = StatusPending
		job.VisibleAt = now
		delete(q.inflight, jobID)
		q.pending[jobID] = job.Clone()
		heap.Push(q.heap, job)
		q.emit(QueueEvent{JobID: jobID, Handler: job.Handler, EventType: EventJobRetrying, Timestamp: now, Error: err.Error()})
		q.logWarn("job retrying", "id", jobID, "attempt", job.RetryCount, "max", job.MaxRetries)
		q.metricCounter("queue.retry.total", 1)
	}

	q.emit(QueueEvent{JobID: jobID, Handler: job.Handler, EventType: EventJobFailed, Timestamp: now, Error: err.Error()})
	q.metricCounter("queue.failed.total", 1)

	if q.store != nil {
		_ = q.store.Update(nil, job)
	}

	return nil
}

func (q *PriorityQueue) promoteDelayed() {
	now := time.Now()
	var remaining []*Job
	for _, job := range q.delayed {
		if !now.Before(job.VisibleAt) {
			job.Status = StatusPending
			cp := job.Clone()
			q.pending[cp.ID] = cp
			heap.Push(q.heap, cp)
			q.emit(QueueEvent{JobID: job.ID, Handler: job.Handler, EventType: EventJobEnqueued, Timestamp: now})
		} else {
			remaining = append(remaining, job)
		}
	}
	q.delayed = remaining
}

func (q *PriorityQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.heap.Len()
}

func (q *PriorityQueue) Pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

func (q *PriorityQueue) InFlight() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.inflight)
}

func (q *PriorityQueue) DeadLettered() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.dead)
}

func (q *PriorityQueue) Delayed() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.delayed)
}

func (q *PriorityQueue) Stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()

	return QueueStats{
		Pending:      len(q.pending),
		Inflight:     len(q.inflight),
		DeadLettered: len(q.dead),
		Delayed:      len(q.delayed),
	}
}

type QueueStats struct {
	Pending      int
	Inflight     int
	DeadLettered int
	Delayed      int
}

func (q *PriorityQueue) DrainDeadLetter(ctx interface{}) ([]*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	jobs := make([]*Job, 0, len(q.dead))
	for _, j := range q.dead {
		jobs = append(jobs, j.Clone())
	}
	q.dead = make(map[string]*Job)
	return jobs, nil
}

func (q *PriorityQueue) RequeueDeadLetter(ctx interface{}, jobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, ok := q.dead[jobID]
	if !ok {
		return nil
	}

	job.RetryCount = 0
	job.Status = StatusPending
	job.VisibleAt = time.Now()
	job.UpdatedAt = time.Now()
	delete(q.dead, jobID)

	q.pending[jobID] = job.Clone()
	heap.Push(q.heap, job)
	q.emit(QueueEvent{JobID: jobID, Handler: job.Handler, EventType: EventJobEnqueued, Timestamp: time.Now()})
	return nil
}

func (q *PriorityQueue) emit(event QueueEvent) {
	select {
	case q.eventCh <- event:
	default:
	}
}

func (q *PriorityQueue) logInfo(msg string, kv ...interface{}) {
	if q.logger != nil {
		q.logger.Info(msg, kv...)
	}
}

func (q *PriorityQueue) logWarn(msg string, kv ...interface{}) {
	if q.logger != nil {
		q.logger.Warn(msg, kv...)
	}
}

func (q *PriorityQueue) metricCounter(name string, value int64) {
	if q.metrics != nil {
		q.metrics.IncrementCounter(name, value)
	}
}

func (q *PriorityQueue) metricDuration(name string, d time.Duration) {
	if q.metrics != nil {
		q.metrics.RecordDuration(name, d)
	}
}
