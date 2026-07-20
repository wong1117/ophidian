package mission

import "github.com/ophidian/ophidian/internal/domain/common"

type Severity = common.Severity

type Phase struct {
	Type      common.Phase
	Status    PhaseStatus
	StartedAt common.UTCTime
	CompletedAt common.UTCTime
	Tasks     []common.ID
}

type PhaseStatus string

const (
	PhasePending   PhaseStatus = "PENDING"
	PhaseRunning   PhaseStatus = "RUNNING"
	PhaseCompleted PhaseStatus = "COMPLETED"
	PhaseFailed    PhaseStatus = "FAILED"
	PhaseSkipped   PhaseStatus = "SKIPPED"
)

type Task struct {
	ID         common.ID
	MissionID  common.ID
	Type       common.TaskType
	Status     common.TaskStatus
	Priority   int
	Parameters map[string]interface{}
	Timeout    int
	RetryCount int
	MaxRetries int
	DependsOn  []common.ID
	AssignedTo string
	CreatedAt  common.UTCTime
	StartedAt  *common.UTCTime
	CompletedAt *common.UTCTime
	Result     *TaskResult
}

type TaskResult struct {
	Status   common.TaskStatus
	Output   []byte
	Evidence []EvidenceRef
	Duration int
	Error    *TaskError
}

type TaskError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

type EvidenceRef struct {
	ID   common.ID
	Type string
	Path string
	Hash string
}
