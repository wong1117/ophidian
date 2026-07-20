package report

import "github.com/ophidian/ophidian/internal/domain/common"

type Report struct {
	ID        common.ID
	MissionID common.ID
	Title     string
	Format    ReportFormat
	Content   []byte
	Summary   ReportSummary
	Status    ReportStatus
	CreatedAt common.UTCTime
}

type ReportFormat string

const (
	FormatJSON     ReportFormat = "JSON"
	FormatMarkdown ReportFormat = "MARKDOWN"
)

type ReportStatus string

const (
	StatusDraft     ReportStatus = "DRAFT"
	StatusGenerated ReportStatus = "GENERATED"
	StatusExported  ReportStatus = "EXPORTED"
)

type ReportSummary struct {
	TotalFindings int
	CriticalCount int
	HighCount     int
	MediumCount   int
	LowCount      int
	TotalDuration int
	SuccessRate   float64
}
