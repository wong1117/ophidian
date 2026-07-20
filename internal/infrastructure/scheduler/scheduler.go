package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/workflow"
)

type JobStore interface {
	Save(ctx context.Context, job *Job) error
	FindByID(ctx context.Context, id string) (*Job, error)
	FindAll(ctx context.Context) ([]*Job, error)
	FindPending(ctx context.Context) ([]*Job, error)
	Update(ctx context.Context, job *Job) error
	Delete(ctx context.Context, id string) error
}

type Scheduler struct {
	mu       sync.Mutex
	jobs     map[string]*Job
	store    JobStore
	logger   Logger
	metrics  Metrics
	eventCh  chan SchedulerEvent
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopCh   chan struct{}
}

type SchedulerOption func(*Scheduler)

func WithLogger(logger Logger) SchedulerOption {
	return func(s *Scheduler) { s.logger = logger }
}

func WithMetrics(metrics Metrics) SchedulerOption {
	return func(s *Scheduler) { s.metrics = metrics }
}

func NewScheduler(store JobStore, opts ...SchedulerOption) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		jobs:    make(map[string]*Job),
		store:   store,
		eventCh: make(chan SchedulerEvent, 100),
		ctx:     ctx,
		cancel:  cancel,
		stopCh:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Scheduler) Events() <-chan SchedulerEvent {
	return s.eventCh
}

func (s *Scheduler) Schedule(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("schedule: job id is required")
	}
	if job.Func == nil {
		return fmt.Errorf("schedule: job func is required for %s", job.ID)
	}

	if job.Backoff == nil {
		job.Backoff = workflow.ExponentialBackoff(time.Second, 2*time.Minute, 0.2)
	}
	if job.Timeout == 0 {
		job.Timeout = 30 * time.Minute
	}

	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	job.Status = StatusPending

	if err := s.computeNextRun(job, now); err != nil {
		return fmt.Errorf("schedule: %w", err)
	}

	jobCopy := *job
	s.jobs[job.ID] = &jobCopy

	if s.store != nil {
		if err := s.store.Save(s.ctx, &jobCopy); err != nil {
			return fmt.Errorf("schedule: persist: %w", err)
		}
	}

	s.emit(SchedulerEvent{JobID: job.ID, EventType: EventJobScheduled, Timestamp: now})
	s.logInfo("job scheduled", "id", job.ID, "type", string(job.ScheduleType), "next_run", job.NextRun.Format(time.RFC3339))

	return nil
}

func (s *Scheduler) computeNextRun(job *Job, now time.Time) error {
	switch job.ScheduleType {
	case ScheduleOnce:
		if job.RunAt.IsZero() {
			return fmt.Errorf("schedule: run_at is required for one-time jobs")
		}
		if job.RunAt.Before(now) {
			job.NextRun = now
		} else {
			job.NextRun = job.RunAt
		}
	case ScheduleRecurring:
		if job.Interval <= 0 {
			return fmt.Errorf("schedule: interval is required for recurring jobs")
		}
		if !job.RunAt.IsZero() && job.RunAt.After(now) {
			job.NextRun = job.RunAt
		} else {
			job.NextRun = now.Add(job.Interval)
		}
	case ScheduleCron:
		if job.CronExpr == "" {
			return fmt.Errorf("schedule: cron expression is required for cron jobs")
		}
		next, err := NextCronTime(job.CronExpr, now)
		if err != nil {
			return fmt.Errorf("schedule: %w", err)
		}
		job.NextRun = next
	default:
		return fmt.Errorf("schedule: unknown schedule type %s", job.ScheduleType)
	}
	return nil
}

func (s *Scheduler) Cancel(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return fmt.Errorf("cancel: job %s not found", jobID)
	}
	if job.Status == StatusRunning {
		return fmt.Errorf("cancel: job %s is currently running", jobID)
	}

	job.Status = StatusCancelled
	job.UpdatedAt = time.Now()
	s.emit(SchedulerEvent{JobID: jobID, EventType: EventJobCancelled, Timestamp: time.Now()})
	return nil
}

func (s *Scheduler) Start() {
	s.logInfo("scheduler started")
	s.wg.Add(1)
	go s.runLoop()
}

func (s *Scheduler) Stop() {
	s.logInfo("scheduler stopping")
	s.cancel()
	s.wg.Wait()
	close(s.stopCh)
	s.logInfo("scheduler stopped")
}

func (s *Scheduler) Recover(ctx context.Context) error {
	if s.store == nil {
		return nil
	}

	jobs, err := s.store.FindPending(ctx)
	if err != nil {
		return fmt.Errorf("recover: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, job := range jobs {
		jobCopy := *job
		s.jobs[job.ID] = &jobCopy
		s.emit(SchedulerEvent{JobID: job.ID, EventType: EventJobRecovered, Timestamp: time.Now()})
	}

	s.logInfo("jobs recovered", "count", len(jobs))
	return nil
}

func (s *Scheduler) runLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processDueJobs()
		}
	}
}

func (s *Scheduler) processDueJobs() {
	s.mu.Lock()
	var dueJobs []*Job
	now := time.Now()
	for _, job := range s.jobs {
		if job.Status == StatusPending && !job.NextRun.After(now) {
			jobCopy := *job
			dueJobs = append(dueJobs, &jobCopy)
		}
	}
	sort.Slice(dueJobs, func(i, j int) bool {
		return dueJobs[i].NextRun.Before(dueJobs[j].NextRun)
	})
	s.mu.Unlock()

	for _, job := range dueJobs {
		s.executeJob(job)
	}
}

func (s *Scheduler) executeJob(job *Job) {
	s.updateJobStatus(job.ID, StatusRunning)

	startedAt := time.Now()
	s.emit(SchedulerEvent{JobID: job.ID, EventType: EventJobStarted, Timestamp: startedAt})
	s.logInfo("job started", "id", job.ID)

	var execErr error
	for attempt := 1; attempt <= job.MaxRetries+1; attempt++ {
		ctx, cancel := context.WithTimeout(s.ctx, job.Timeout)
		execErr = job.Func(ctx)
		cancel()

		if execErr == nil {
			break
		}

		if attempt <= job.MaxRetries {
			delay := job.Backoff(attempt)
			s.emit(SchedulerEvent{JobID: job.ID, EventType: EventJobRetrying, Error: execErr.Error(), Timestamp: time.Now()})
			s.logWarn("job retrying", "id", job.ID, "attempt", attempt, "max", job.MaxRetries, "delay_ms", delay.Milliseconds())
			s.metricCounter("scheduler.job.retry", 1)

			select {
			case <-time.After(delay):
			case <-s.ctx.Done():
				execErr = s.ctx.Err()
				goto done
			}
		}
	}

done:
	s.mu.Lock()
	current, ok := s.jobs[job.ID]
	if !ok {
		s.mu.Unlock()
		return
	}

	duration := time.Since(startedAt)
	now := time.Now()
	totalAttempts := job.Attempts + (job.MaxRetries + 1)
	if totalAttempts < 1 {
		totalAttempts = 1
	}
	current.Attempts = totalAttempts
	_ = duration
	current.LastRun = &now
	current.UpdatedAt = now

	if execErr != nil {
		current.Status = StatusFailed
		current.LastError = execErr.Error()
		s.emit(SchedulerEvent{JobID: job.ID, EventType: EventJobFailed, Error: execErr.Error(), Timestamp: now})
		s.logError("job failed", "id", job.ID, "error", execErr.Error())
		s.metricCounter("scheduler.job.failed", 1)
	} else {
		current.Status = StatusCompleted
		s.emit(SchedulerEvent{JobID: job.ID, EventType: EventJobCompleted, Timestamp: now})
		s.logInfo("job completed", "id", job.ID)
		s.metricCounter("scheduler.job.completed", 1)
	}

	if current.ScheduleType == ScheduleRecurring || current.ScheduleType == ScheduleCron {
		current.Status = StatusPending
		if err := s.computeNextRun(current, now); err != nil {
			current.Status = StatusFailed
			current.LastError = err.Error()
		}
	}

	if s.store != nil {
		if err := s.store.Update(s.ctx, current); err != nil {
			s.logError("job persist failed", "id", job.ID, "error", err.Error())
		}
	}

	s.metricDuration("scheduler.job.duration", duration)
	s.mu.Unlock()
}

func (s *Scheduler) updateJobStatus(jobID string, status JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[jobID]; ok {
		job.Status = status
		job.UpdatedAt = time.Now()
	}
}

func (s *Scheduler) List() []*Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobCopy := *j
		result = append(result, &jobCopy)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NextRun.Before(result[j].NextRun)
	})
	return result
}

func (s *Scheduler) Get(jobID string) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return nil, false
	}
	jobCopy := *job
	return &jobCopy, true
}

func (s *Scheduler) emit(event SchedulerEvent) {
	select {
	case s.eventCh <- event:
	default:
	}
}

func (s *Scheduler) logInfo(msg string, kv ...interface{}) {
	if s.logger != nil {
		s.logger.Info(msg, kv...)
	}
}

func (s *Scheduler) logWarn(msg string, kv ...interface{}) {
	if s.logger != nil {
		s.logger.Warn(msg, kv...)
	}
}

func (s *Scheduler) logError(msg string, kv ...interface{}) {
	if s.logger != nil {
		s.logger.Error(msg, kv...)
	}
}

func (s *Scheduler) metricCounter(name string, value int64) {
	if s.metrics != nil {
		s.metrics.IncrementCounter(name, value)
	}
}

func (s *Scheduler) metricDuration(name string, d time.Duration) {
	if s.metrics != nil {
		s.metrics.RecordDuration(name, d)
	}
}

func attemptHigh(a, b int) int {
	if a > b {
		return a
	}
	return b
}
