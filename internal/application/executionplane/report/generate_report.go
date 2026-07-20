package report

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type ReportGenerator interface {
	Generate(ctx context.Context, data ReportData, format string) ([]byte, error)
}

type ReportData struct {
	MissionID   string
	Targets     []string
	Findings    []finding.Finding
	Timeline    []TimelineEntry
	Summary     ReportSummary
}

type ReportSummary struct {
	TotalFindings   int
	CriticalCount   int
	HighCount       int
	MediumCount     int
	LowCount        int
	TotalDuration   int
	SuccessRate     float64
}

type TimelineEntry struct {
	Timestamp int64
	Event     string
	Detail    string
}

type GenerateReportUseCase struct {
	findingRepo finding.FindingRepository
	generator   ReportGenerator
}

func NewGenerateReportUseCase(fr finding.FindingRepository, gen ReportGenerator) *GenerateReportUseCase {
	return &GenerateReportUseCase{findingRepo: fr, generator: gen}
}
