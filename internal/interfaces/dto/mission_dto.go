package dto

type CreateMissionRequest struct {
	Name       string        `json:"name"`
	Target     TargetDTO     `json:"target"`
	Objectives []ObjectiveDTO `json:"objectives"`
	RoE        RoEDTO        `json:"roe"`
	StartedBy  string        `json:"started_by"`
}

type TargetDTO struct {
	Name    string   `json:"name"`
	IPs     []string `json:"ips"`
	Domains []string `json:"domains"`
	CIDRs   []string `json:"cidrs"`
}

type ObjectiveDTO struct {
	Description string `json:"description"`
	Priority    int    `json:"priority"`
}

type RoEDTO struct {
	MaxSeverity      string `json:"max_severity"`
	AllowDestructive bool   `json:"allow_destructive"`
	AllowPersistence bool   `json:"allow_persistence"`
	AllowExfiltration bool  `json:"allow_exfiltration"`
	MaxTargets       int    `json:"max_targets"`
}

type MissionResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}
