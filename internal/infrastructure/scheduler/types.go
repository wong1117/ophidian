package scheduler

import (
	"context"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/workflow"
)

type JobStatus string

const (
	StatusPending   JobStatus = "PENDING"
	StatusRunning   JobStatus = "RUNNING"
	StatusCompleted JobStatus = "COMPLETED"
	StatusFailed    JobStatus = "FAILED"
	StatusCancelled JobStatus = "CANCELLED"
)

type ScheduleType string

const (
	ScheduleOnce     ScheduleType = "ONCE"
	ScheduleRecurring ScheduleType = "RECURRING"
	ScheduleCron     ScheduleType = "CRON"
)

type Job struct {
	ID           string
	Name         string
	ScheduleType ScheduleType
	CronExpr     string
	RunAt        time.Time
	Interval     time.Duration
	Func         JobFunc
	Timeout      time.Duration
	MaxRetries   int
	Backoff      workflow.BackoffFunc
	NextRun      time.Time
	LastRun      *time.Time
	Status       JobStatus
	Attempts     int
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type JobFunc func(ctx context.Context) error

type JobResult struct {
	JobID     string
	Status    JobStatus
	Error     string
	Attempts  int
	Duration  time.Duration
	StartedAt time.Time
	EndedAt   time.Time
}

type SchedulerEvent struct {
	JobID     string
	EventType SchedulerEventType
	Status    JobStatus
	Error     string
	Timestamp time.Time
}

type SchedulerEventType string

const (
	EventJobScheduled  SchedulerEventType = "JOB_SCHEDULED"
	EventJobStarted    SchedulerEventType = "JOB_STARTED"
	EventJobCompleted  SchedulerEventType = "JOB_COMPLETED"
	EventJobFailed     SchedulerEventType = "JOB_FAILED"
	EventJobRetrying   SchedulerEventType = "JOB_RETRYING"
	EventJobRecovered  SchedulerEventType = "JOB_RECOVERED"
	EventJobCancelled  SchedulerEventType = "JOB_CANCELLED"
)

type Logger interface {
	Debug(msg string, kv ...interface{})
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
}

type Metrics interface {
	IncrementCounter(name string, value int64)
	RecordDuration(name string, d time.Duration)
}
