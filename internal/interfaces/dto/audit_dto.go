package dto

import "time"

type AuditEntry struct {
	ID            string                 `json:"id"`
	AggregateID   string                 `json:"aggregate_id"`
	AggregateType string                 `json:"aggregate_type"`
	EventType     string                 `json:"event_type"`
	Payload       map[string]interface{} `json:"payload"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	OccurredAt    time.Time              `json:"occurred_at"`
}

type AuditFilter struct {
	TenantID      string     `json:"tenant_id,omitempty"`
	UserID        string     `json:"user_id,omitempty"`
	AggregateType string     `json:"aggregate_type,omitempty"`
	EventType     string     `json:"event_type,omitempty"`
	Search        string     `json:"search,omitempty"`
	From          *time.Time `json:"from,omitempty"`
	To            *time.Time `json:"to,omitempty"`
	Page          int        `json:"page"`
	PerPage       int        `json:"per_page"`
}

type AuditResponse struct {
	Entries    []AuditEntry `json:"entries"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalPages int          `json:"total_pages"`
}

type AuditMetrics struct {
	TotalEvents      int64            `json:"total_events"`
	EventsByType     map[string]int64 `json:"events_by_type"`
	EventsByAggregate map[string]int64 `json:"events_by_aggregate"`
	RecentActivity   int64            `json:"recent_activity_24h"`
}
