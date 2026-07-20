package dto

type StartReconRequest struct {
	TargetID string `json:"target_id"`
	Ports    string `json:"ports,omitempty"`
	Type     string `json:"type"`
}

type ReconResultResponse struct {
	TargetID  string        `json:"target_id"`
	IPs       []string      `json:"ips"`
	Domains   []string      `json:"domains"`
	Services  []ServiceDTO  `json:"services"`
	OS        string        `json:"os"`
	Status    string        `json:"status"`
}

type ServiceDTO struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Banner   string `json:"banner"`
}
