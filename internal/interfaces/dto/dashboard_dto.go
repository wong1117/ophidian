package dto

import "time"

type DashboardOverview struct {
	Missions       MissionStats       `json:"missions"`
	Workflows      WorkflowStats      `json:"workflows"`
	Findings       FindingStats       `json:"findings"`
	Queues         QueueStats         `json:"queues"`
	Workers        WorkerStats        `json:"workers"`
	System         SystemMetrics      `json:"system"`
	GeneratedAt    time.Time          `json:"generated_at"`
}

type MissionStats struct {
	Total       int     `json:"total"`
	Active      int     `json:"active"`
	Completed   int     `json:"completed"`
	Failed      int     `json:"failed"`
	Paused      int     `json:"paused"`
	SuccessRate float64 `json:"success_rate"`
	AvgDuration int64   `json:"avg_duration_ms"`
}

type WorkflowStats struct {
	Total         int     `json:"total"`
	Running       int     `json:"running"`
	Completed     int     `json:"completed"`
	Failed        int     `json:"failed"`
	Cancelled     int     `json:"cancelled"`
	SuccessRate   float64 `json:"success_rate"`
	AvgDuration   int64   `json:"avg_duration_ms"`
	ActiveWorkers int     `json:"active_workers"`
}

type FindingStats struct {
	Total    int     `json:"total"`
	Critical int     `json:"critical"`
	High     int     `json:"high"`
	Medium   int     `json:"medium"`
	Low      int     `json:"low"`
	AvgCVSS  float64 `json:"avg_cvss"`
}

type QueueStats struct {
	Pending      int `json:"pending"`
	Inflight     int `json:"inflight"`
	DeadLettered int `json:"dead_lettered"`
	Delayed      int `json:"delayed"`
	Completed    int `json:"completed"`
}

type WorkerStats struct {
	Total   int     `json:"total"`
	Idle    int     `json:"idle"`
	Busy    int     `json:"busy"`
	Offline int     `json:"offline"`
	Uptime  float64 `json:"uptime_pct"`
}

type SystemMetrics struct {
	Uptime            string  `json:"uptime"`
	CPUUsage          float64 `json:"cpu_usage_pct"`
	MemoryUsage       uint64  `json:"memory_usage_bytes"`
	Goroutines        int     `json:"goroutines"`
	HTTPRequests      int64   `json:"http_requests_total"`
	HTTPErrors        int64   `json:"http_errors_total"`
	AvgResponseTimeMs float64 `json:"avg_response_time_ms"`
}

type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	Detail    string    `json:"detail"`
}

type DashboardTimeline struct {
	Entries    []TimelineEntry `json:"entries"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
}

type DashboardFilter struct {
	TenantID string     `json:"tenant_id,omitempty"`
	From     *time.Time `json:"from,omitempty"`
	To       *time.Time `json:"to,omitempty"`
	Status   string     `json:"status,omitempty"`
	Limit    int        `json:"limit,omitempty"`
}
