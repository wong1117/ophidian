package dto

type GeneratePlanRequest struct {
	MissionID   string           `json:"mission_id"`
	TargetData  TargetProfileDTO `json:"target_data"`
	Constraints RoEDTO           `json:"constraints"`
}

type TargetProfileDTO struct {
	IPs       []string     `json:"ips"`
	Domains   []string     `json:"domains"`
	Services  []ServiceDTO `json:"services"`
	OS        string       `json:"os"`
	OpenPorts []int        `json:"open_ports"`
}

type PlanResponseDTO struct {
	PlanID      string          `json:"plan_id"`
	RankedPaths []RankedPathDTO `json:"ranked_paths"`
	Confidence  float64         `json:"confidence"`
	Rationale   string          `json:"rationale"`
}

type RankedPathDTO struct {
	Steps      []string `json:"steps"`
	Score      float64  `json:"score"`
	Confidence float64  `json:"confidence"`
	RiskLevel  string   `json:"risk_level"`
}

type AttackPlanGeneratedResponse struct {
	PlanID      string          `json:"plan_id"`
	MissionID   string          `json:"mission_id"`
	Nodes       []NodeDTO       `json:"nodes"`
	Edges       []EdgeDTO       `json:"edges"`
	RankedPaths []RankedPathDTO `json:"ranked_paths"`
	Confidence  float64         `json:"confidence"`
	Rationale   string          `json:"rationale"`
	ETA         int             `json:"eta"`
	Status      string          `json:"status"`
	CreatedAt   string          `json:"created_at"`
}

type NodeDTO struct {
	ID         string  `json:"id"`
	TargetID   string  `json:"target_id"`
	Type       string  `json:"type"`
	Service    string  `json:"service"`
	CVE        string  `json:"cve"`
	Confidence float64 `json:"confidence"`
	RiskLevel  string  `json:"risk_level"`
}

type EdgeDTO struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Weight     float64 `json:"weight"`
	Confidence float64 `json:"confidence"`
	Condition  string  `json:"condition"`
}


