package attackplan

import "context"

type AIPlanner interface {
	GeneratePlan(ctx context.Context, req PlanRequest) (*PlanResponse, error)
	CorrelateFindings(ctx context.Context, findings []Finding) (*CorrelationResult, error)
	RankPaths(ctx context.Context, graph AttackGraph) ([]RankedPath, error)
	AdaptStrategy(ctx context.Context, planID string, feedback StrategyFeedback) (*Strategy, error)
	EvaluateConfidence(ctx context.Context, plan *AttackPlan, evidence []Evidence) (float64, error)
}

type PlanRequest struct {
	MissionID  string
	TargetData TargetProfile
	Constraints RoEConstraints
	History    []PastAttempt
}

type PlanResponse struct {
	PlanID      string
	Graph       AttackGraph
	RankedPaths []RankedPath
	Confidence  float64
	Rationale   string
	ETA         int
}

type TargetProfile struct {
	IPs       []string
	Domains   []string
	Services  []ServiceInfo
	OS        string
	OpenPorts []int
	Tags      []string
}

type ServiceInfo struct {
	Port     int
	Protocol string
	Name     string
	Version  string
	Banner   string
}

type RoEConstraints struct {
	MaxSeverity      string
	AllowDestructive bool
	AllowPersistence bool
	AllowExfiltration bool
}

type PastAttempt struct {
	TaskID    string
	TargetID  string
	Timestamp int64
	Result    string
	Error     string
}

type StrategyFeedback struct {
	PlanID      string
	StepResults []StepResult
	Overall     string
}

type StepResult struct {
	StepID string
	Status string
	Error  string
}

type Evidence struct {
	ID      string
	Type    string
	Content string
	Source  string
}

type Finding struct {
	ID          string
	Title       string
	Description string
	Severity    string
	CVE         string
	Confidence  string
}

type CorrelationResult struct {
	Groups      []FindingGroup
	Confidence  float64
	Rationale   string
}

type FindingGroup struct {
	ID          string
	Findings    []Finding
	CommonCVE   string
	CommonTech  string
	RiskScore   float64
}
