package mission

import "github.com/ophidian/ophidian/internal/domain/common"

type Mission struct {
	ID         common.ID
	Name       string
	Target     Target
	Objectives []Objective
	RoE        RoEConstraints
	Status     MissionStatus
	Phases     []Phase
	Tasks      []Task
	CreatedAt  common.UTCTime
	UpdatedAt  common.UTCTime
	StartedBy  string
}

type MissionStatus string

const (
	MissionDraft     MissionStatus = "DRAFT"
	MissionActive    MissionStatus = "ACTIVE"
	MissionPaused    MissionStatus = "PAUSED"
	MissionCompleted MissionStatus = "COMPLETED"
	MissionAborted   MissionStatus = "ABORTED"
	MissionFailed    MissionStatus = "FAILED"
)

type Target struct {
	ID        common.ID
	Name      string
	IPs       []string
	Domains   []string
	CIDRs     []string
	Scope     Scope
	CreatedAt common.UTCTime
}

type Scope struct {
	AllowedCIDRs   []string
	AllowedDomains []string
	ExcludedCIDRs  []string
	ExcludedHosts  []string
}

type Objective struct {
	ID          common.ID
	Description string
	Priority    int
	Completed   bool
}

type RoEConstraints struct {
	MaxSeverity      Severity
	AllowDestructive bool
	AllowPersistence bool
	AllowExfiltration bool
	TimeWindowStart  common.UTCTime
	TimeWindowEnd    common.UTCTime
	MaxTargets       int
	ExcludedNets     []string
	AllowedTechs     []string
	RequireApproval  bool
}

type PastAttempt struct {
	TaskID    common.ID
	TargetID  common.ID
	Timestamp common.UTCTime
	Result    string
	Error     string
}
