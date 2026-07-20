package attackplan

import "github.com/ophidian/ophidian/internal/domain/common"

type AttackPlan struct {
	ID          common.ID
	MissionID   common.ID
	Graph       AttackGraph
	RankedPaths []RankedPath
	Confidence  float64
	Rationale   string
	ETA         int
	Status      PlanStatus
	CreatedAt   common.UTCTime
	UpdatedAt   common.UTCTime
}

type PlanStatus string

const (
	PlanDraft     PlanStatus = "DRAFT"
	PlanActive    PlanStatus = "ACTIVE"
	PlanCompleted PlanStatus = "COMPLETED"
	PlanRejected  PlanStatus = "REJECTED"
	PlanFailed    PlanStatus = "FAILED"
)

type AttackGraph struct {
	Nodes []Node
	Edges []Edge
}

type Node struct {
	ID         string
	TargetID   string
	Type       NodeType
	Service    string
	CVE        string
	Confidence float64
	RiskLevel  common.RiskLevel
	Metadata   map[string]interface{}
}

type NodeType string

const (
	NodeRecon   NodeType = "RECON"
	NodeExploit NodeType = "EXPLOIT"
	NodePostExp NodeType = "POST_EXPLOIT"
	NodePivot   NodeType = "PIVOT"
)

type Edge struct {
	From       string
	To         string
	Weight     float64
	Confidence float64
	Condition  string
}

type RankedPath struct {
	Nodes      []string
	TotalScore float64
	Confidence float64
	RiskLevel  common.RiskLevel
	Steps      int
}
