package common

type PlaneType string

const (
	PlaneControl   PlaneType = "CONTROL"
	PlaneAI        PlaneType = "AI"
	PlaneExecution PlaneType = "EXECUTION"
)

type TaskType string

const (
	TaskRecon       TaskType = "RECON"
	TaskExploit     TaskType = "EXPLOIT"
	TaskPostExploit TaskType = "POSTEXPLOIT"
	TaskReport      TaskType = "REPORT"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "PENDING"
	TaskRunning    TaskStatus = "RUNNING"
	TaskSuccess    TaskStatus = "SUCCESS"
	TaskFailed     TaskStatus = "FAILED"
	TaskTimeout    TaskStatus = "TIMEOUT"
	TaskCancelled  TaskStatus = "CANCELLED"
	TaskPartial    TaskStatus = "PARTIAL"
)

type Phase string

const (
	PhaseRecon       Phase = "RECON"
	PhasePlanning    Phase = "PLANNING"
	PhaseExploit     Phase = "EXPLOIT"
	PhasePostExploit Phase = "POSTEXPLOIT"
	PhaseReport      Phase = "REPORT"
	PhaseComplete    Phase = "COMPLETE"
	PhaseAborted     Phase = "ABORTED"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

type PlanDecision string

const (
	PlanAccepted PlanDecision = "ACCEPTED"
	PlanRejected PlanDecision = "REJECTED"
	PlanModified PlanDecision = "MODIFIED"
)

type RiskLevel string

const (
	RiskCritical RiskLevel = "CRITICAL"
	RiskHigh     RiskLevel = "HIGH"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskLow      RiskLevel = "LOW"
)
