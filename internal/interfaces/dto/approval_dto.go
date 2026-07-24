package dto

import "time"

type RecommendationDTO struct {
	EventID      string    `json:"event_id"`
	MissionID    string    `json:"mission_id"`
	SourceEventID string   `json:"source_event_id"`
	Recommendation string  `json:"recommendation"`
	Confidence   float64   `json:"confidence"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type ApprovalRequestDTO struct {
	MissionID     string `json:"mission_id"`
	SourceEventID string `json:"source_event_id"`
	Decision      string `json:"decision"`
	OperatorNote  string `json:"operator_note"`
}

type ApprovalResponseDTO struct {
	Status  string `json:"status"`
	EventID string `json:"event_id"`
}
