package dto

type GenerateReportRequest struct {
	MissionID string `json:"mission_id"`
	Format    string `json:"format"`
}

type ReportResponse struct {
	ReportID  string           `json:"report_id"`
	MissionID string           `json:"mission_id"`
	Title     string           `json:"title"`
	Format    string           `json:"format"`
	Data      []byte           `json:"data"`
	Filename  string           `json:"filename"`
	Summary   ReportSummaryDTO `json:"summary"`
}

type ReportSummaryDTO struct {
	TotalFindings int     `json:"total_findings"`
	CriticalCount int     `json:"critical_count"`
	HighCount     int     `json:"high_count"`
	MediumCount   int     `json:"medium_count"`
	LowCount      int     `json:"low_count"`
	TotalDuration int     `json:"total_duration"`
	SuccessRate   float64 `json:"success_rate"`
}
