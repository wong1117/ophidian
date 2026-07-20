package dto

type GeneratePlanRequest struct {
	MissionID   string   `json:"mission_id"`
	TargetData  TargetProfileDTO `json:"target_data"`
	Constraints RoEDTO   `json:"constraints"`
}

type TargetProfileDTO struct {
	IPs       []string      `json:"ips"`
	Domains   []string      `json:"domains"`
	Services  []ServiceDTO  `json:"services"`
	OS        string        `json:"os"`
	OpenPorts []int         `json:"open_ports"`
}

type PlanResponseDTO struct {
	PlanID      string           `json:"plan_id"`
	RankedPaths []RankedPathDTO  `json:"ranked_paths"`
	Confidence  float64          `json:"confidence"`
	Rationale   string           `json:"rationale"`
}

type RankedPathDTO struct {
	Steps      []string `json:"steps"`
	Score      float64  `json:"score"`
	Confidence float64  `json:"confidence"`
	RiskLevel  string   `json:"risk_level"`
}
