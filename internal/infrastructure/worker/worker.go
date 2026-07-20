package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/queue"
)

type WorkerStatus string

const (
	WorkerStatusIdle      WorkerStatus = "IDLE"
	WorkerStatusBusy      WorkerStatus = "BUSY"
	WorkerStatusOffline   WorkerStatus = "OFFLINE"
	WorkerStatusDraining  WorkerStatus = "DRAINING"
)

type Worker struct {
	ID           string
	Capabilities []string
	Status       WorkerStatus
	LastSeen     time.Time
	CurrentJob   *queue.Job
	Labels       map[string]string
}

type WorkerEventType string

const (
	EventWorkerRegistered   WorkerEventType = "WORKER_REGISTERED"
	EventWorkerHeartbeat    WorkerEventType = "WORKER_HEARTBEAT"
	EventWorkerOffline      WorkerEventType = "WORKER_OFFLINE"
	EventWorkerDraining     WorkerEventType = "WORKER_DRAINING"
	EventJobAssigned        WorkerEventType = "WORKER_JOB_ASSIGNED"
	EventJobCompleted       WorkerEventType = "WORKER_JOB_COMPLETED"
	EventJobFailed          WorkerEventType = "WORKER_JOB_FAILED"
	EventJobReassigned      WorkerEventType = "WORKER_JOB_REASSIGNED"
)

type WorkerEvent struct {
	WorkerID  string
	EventType WorkerEventType
	JobID     string
	Timestamp time.Time
	Error     string
}

type JobQueue interface {
	Dequeue(ctx interface{}) (*queue.Job, error)
	Ack(ctx interface{}, jobID string) error
	Nack(ctx interface{}, jobID string, err error) error
}

type JobHandler func(ctx context.Context, job *queue.Job) error

type Logger interface {
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
}

type Metrics interface {
	IncrementCounter(name string, value int64)
	RecordDuration(name string, d time.Duration)
}

type WorkerPool struct {
	mu            sync.RWMutex
	workers       map[string]*Worker
	queue         JobQueue
	handlers      map[string]JobHandler
	logger        Logger
	metrics       Metrics
	eventCh       chan WorkerEvent
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	heartbeatTimeout time.Duration
	heartbeatInterval time.Duration
	pollInterval     time.Duration
	maxConcurrency   int
	sem              chan struct{}
}

type PoolOption func(*WorkerPool)

func WithLogger(logger Logger) PoolOption {
	return func(p *WorkerPool) { p.logger = logger }
}

func WithMetrics(metrics Metrics) PoolOption {
	return func(p *WorkerPool) { p.metrics = metrics }
}

func WithHeartbeatTimeout(d time.Duration) PoolOption {
	return func(p *WorkerPool) { p.heartbeatTimeout = d }
}

func WithHeartbeatInterval(d time.Duration) PoolOption {
	return func(p *WorkerPool) { p.heartbeatInterval = d }
}

func WithPollInterval(d time.Duration) PoolOption {
	return func(p *WorkerPool) { p.pollInterval = d }
}

func WithMaxConcurrency(n int) PoolOption {
	return func(p *WorkerPool) { p.maxConcurrency = n; p.sem = make(chan struct{}, n) }
}

func NewWorkerPool(q JobQueue, opts ...PoolOption) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &WorkerPool{
		workers:           make(map[string]*Worker),
		queue:             q,
		handlers:          make(map[string]JobHandler),
		eventCh:           make(chan WorkerEvent, 100),
		ctx:               ctx,
		cancel:            cancel,
		heartbeatTimeout:  30 * time.Second,
		heartbeatInterval: 5 * time.Second,
		pollInterval:      500 * time.Millisecond,
		maxConcurrency:    10,
		sem:               make(chan struct{}, 10),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *WorkerPool) Events() <-chan WorkerEvent {
	return p.eventCh
}

func (p *WorkerPool) RegisterHandler(name string, handler JobHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[name] = handler
}

func (p *WorkerPool) RegisterWorker(workerID string, capabilities []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := &Worker{
		ID:           workerID,
		Capabilities: capabilities,
		Status:       WorkerStatusIdle,
		LastSeen:     time.Now(),
	}
	p.workers[workerID] = w

	p.emit(WorkerEvent{WorkerID: workerID, EventType: EventWorkerRegistered, Timestamp: time.Now()})
	p.logInfo("worker registered", "id", workerID, "capabilities", fmt.Sprintf("%v", capabilities))
	p.metricCounter("worker.registered.total", 1)
}

func (p *WorkerPool) DeregisterWorker(workerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, ok := p.workers[workerID]; ok {
		w.Status = WorkerStatusOffline
		jobID := ""
		if w.CurrentJob != nil {
			jobID = w.CurrentJob.ID
			p.emit(WorkerEvent{WorkerID: workerID, EventType: EventJobReassigned, JobID: jobID, Timestamp: time.Now()})
		}
		p.emit(WorkerEvent{WorkerID: workerID, EventType: EventWorkerOffline, Timestamp: time.Now()})
		delete(p.workers, workerID)
	}
	p.metricCounter("worker.offline.total", 1)
}

func (p *WorkerPool) Heartbeat(workerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, ok := p.workers[workerID]; ok {
		w.LastSeen = time.Now()
		p.emit(WorkerEvent{WorkerID: workerID, EventType: EventWorkerHeartbeat, Timestamp: time.Now()})
	}
}

func (p *WorkerPool) Start() {
	p.logInfo("worker pool started", "workers", len(p.workers))
	p.wg.Add(1)
	go p.pollLoop()
	p.wg.Add(1)
	go p.healthCheckLoop()
}

func (p *WorkerPool) Stop() {
	p.logInfo("worker pool stopping")
	p.cancel()
	p.wg.Wait()
	p.logInfo("worker pool stopped")
}

func (p *WorkerPool) pollLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.dispatchJobs()
		}
	}
}

func (p *WorkerPool) healthCheckLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.checkHealth()
		}
	}
}

func (p *WorkerPool) dispatchJobs() {
	p.sem <- struct{}{}
	defer func() { <-p.sem }()

	job, err := p.queue.Dequeue(nil)
	if err != nil || job == nil {
		return
	}

	p.mu.RLock()
	handler, handlerExists := p.handlers[job.Handler]
	var available *Worker
	for _, w := range p.workers {
		if w.Status == WorkerStatusIdle && p.hasCapability(w, job.Handler) {
			available = w
			break
		}
	}
	p.mu.RUnlock()

	if !handlerExists || available == nil {
		_ = p.queue.Nack(nil, job.ID, fmt.Errorf("no worker available for handler %s", job.Handler))
		return
	}

	p.mu.Lock()
	available.Status = WorkerStatusBusy
	available.CurrentJob = job
	p.mu.Unlock()

	start := time.Now()
	err = handler(p.ctx, job)

	p.mu.Lock()
	available.Status = WorkerStatusIdle
	available.CurrentJob = nil
	p.mu.Unlock()

	if err != nil {
		p.queue.Nack(nil, job.ID, err)
		p.emit(WorkerEvent{WorkerID: available.ID, EventType: EventJobFailed, JobID: job.ID, Timestamp: time.Now(), Error: err.Error()})
		p.metricCounter("worker.job.failed", 1)
	} else {
		p.queue.Ack(nil, job.ID)
		p.emit(WorkerEvent{WorkerID: available.ID, EventType: EventJobCompleted, JobID: job.ID, Timestamp: time.Now()})
		p.metricCounter("worker.job.completed", 1)
		p.metricDuration("worker.job.duration", time.Since(start))
	}
}

func (p *WorkerPool) checkHealth() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for id, w := range p.workers {
		if now.Sub(w.LastSeen) > p.heartbeatTimeout {
			w.Status = WorkerStatusOffline
			p.metricCounter("worker.offline.total", 1)
			p.emit(WorkerEvent{WorkerID: id, EventType: EventWorkerOffline, Timestamp: now})
			p.logWarn("worker offline", "id", id, "last_seen", w.LastSeen.Format(time.RFC3339))
		}
	}
}

func (p *WorkerPool) Drain() {
	p.mu.Lock()
	for _, w := range p.workers {
		w.Status = WorkerStatusDraining
		p.emit(WorkerEvent{WorkerID: w.ID, EventType: EventWorkerDraining, Timestamp: time.Now()})
	}
	p.mu.Unlock()

	p.logInfo("worker pool draining", "count", len(p.workers))
}

func (p *WorkerPool) Workers() []*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*Worker, 0, len(p.workers))
	for _, w := range p.workers {
		cp := *w
		if cp.CurrentJob != nil {
			jobCopy := *cp.CurrentJob
			cp.CurrentJob = &jobCopy
		}
		result = append(result, &cp)
	}
	return result
}

func (p *WorkerPool) hasCapability(w *Worker, handler string) bool {
	for _, c := range w.Capabilities {
		if c == handler {
			return true
		}
	}
	return false
}

func (p *WorkerPool) emit(event WorkerEvent) {
	select {
	case p.eventCh <- event:
	default:
	}
}

func (p *WorkerPool) logInfo(msg string, kv ...interface{}) {
	if p.logger != nil {
		p.logger.Info(msg, kv...)
	}
}

func (p *WorkerPool) logWarn(msg string, kv ...interface{}) {
	if p.logger != nil {
		p.logger.Warn(msg, kv...)
	}
}

func (p *WorkerPool) metricCounter(name string, value int64) {
	if p.metrics != nil {
		p.metrics.IncrementCounter(name, value)
	}
}

func (p *WorkerPool) metricDuration(name string, d time.Duration) {
	if p.metrics != nil {
		p.metrics.RecordDuration(name, d)
	}
}
