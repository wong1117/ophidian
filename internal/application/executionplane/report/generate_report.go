package report

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	domainReport "github.com/ophidian/ophidian/internal/domain/report"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type ReportData struct {
	MissionID string
	Targets     []string
	Findings    []finding.Finding
	Timeline    []TimelineEntry
	Summary     ReportSummary
}

type ReportSummary struct {
	TotalFindings int
	CriticalCount int
	HighCount     int
	MediumCount   int
	LowCount      int
	TotalDuration int
	SuccessRate   float64
}

type TimelineEntry struct {
	Timestamp int64
	Event     string
	Detail    string
}

type ReportFormatter interface {
	Format(data *ReportData) ([]byte, error)
}

type EventStore interface {
	Append(ctx context.Context, event interface{}) error
}

type ReportRepository interface {
	Save(ctx context.Context, r *domainReport.Report) error
}

type GenerateReportUseCase struct {
	findingRepo finding.FindingRepository
	reportRepo  ReportRepository
	eventStore  EventStore
	jsonFmt     ReportFormatter
	markdownFmt ReportFormatter
}

func NewGenerateReportUseCase(
	findingRepo finding.FindingRepository,
	reportRepo ReportRepository,
	eventStore EventStore,
) *GenerateReportUseCase {
	return &GenerateReportUseCase{
		findingRepo: findingRepo,
		reportRepo:  reportRepo,
		eventStore:  eventStore,
		jsonFmt:     &JSONFormatter{},
		markdownFmt: &MarkdownFormatter{},
	}
}

type GenerateReportRequest struct {
	MissionID string
	Format    string
}

type GenerateReportResponse struct {
	Report *dto.ReportResponse
}

func (uc *GenerateReportUseCase) Execute(ctx context.Context, req GenerateReportRequest) (*GenerateReportResponse, error) {
	if req.MissionID == "" {
		return nil, fmt.Errorf("%w: mission id is required", common.ErrInvalidID)
	}

	format := domainReport.ReportFormat(strings.ToUpper(req.Format))
	if format != domainReport.FormatJSON && format != domainReport.FormatMarkdown {
		format = domainReport.FormatJSON
	}

	data, err := uc.collectReportData(ctx, req.MissionID)
	if err != nil {
		return nil, fmt.Errorf("collect report data: %w", err)
	}

	content, err := uc.formatReport(format, data)
	if err != nil {
		return nil, fmt.Errorf("format report: %w", err)
	}

	reportEntity := &domainReport.Report{
		ID:        common.NewID(),
		MissionID: common.ID(req.MissionID),
		Title:     fmt.Sprintf("Mission Report - %s", req.MissionID),
		Format:    format,
		Content:   content,
		Summary: domainReport.ReportSummary{
			TotalFindings: data.Summary.TotalFindings,
			CriticalCount: data.Summary.CriticalCount,
			HighCount:     data.Summary.HighCount,
			MediumCount:   data.Summary.MediumCount,
			LowCount:      data.Summary.LowCount,
			TotalDuration: data.Summary.TotalDuration,
			SuccessRate:   data.Summary.SuccessRate,
		},
		Status:    domainReport.StatusGenerated,
		CreatedAt: common.Now(),
	}

	if err := uc.reportRepo.Save(ctx, reportEntity); err != nil {
		return nil, fmt.Errorf("save report: %w", err)
	}

	event := domainReport.ReportGenerated{
		ReportID:  reportEntity.ID,
		MissionID: reportEntity.MissionID,
		Format:    format,
		Timestamp: common.Now(),
	}
	if err := uc.eventStore.Append(ctx, event); err != nil {
		return nil, fmt.Errorf("append report event: %w", err)
	}

	return &GenerateReportResponse{
		Report: mapToReportDTO(reportEntity),
	}, nil
}

func (uc *GenerateReportUseCase) collectReportData(ctx context.Context, missionID string) (*ReportData, error) {
	findings, err := uc.findingRepo.FindByMission(ctx, missionID)
	if err != nil {
		return nil, fmt.Errorf("fetch findings: %w", err)
	}

	var targets []string
	for _, f := range findings {
		targets = append(targets, f.TargetID.String())
	}
	targets = uniqueStrings(targets)

	domainFindings := make([]finding.Finding, len(findings))
	for i, f := range findings {
		domainFindings[i] = *f
	}

	summary := buildSummary(domainFindings)
	timeline := buildTimeline(domainFindings)

	return &ReportData{
		MissionID: missionID,
		Targets:   targets,
		Findings:  domainFindings,
		Timeline:  timeline,
		Summary:   summary,
	}, nil
}

func (uc *GenerateReportUseCase) formatReport(format domainReport.ReportFormat, data *ReportData) ([]byte, error) {
	switch format {
	case domainReport.FormatMarkdown:
		return uc.markdownFmt.Format(data)
	default:
		return uc.jsonFmt.Format(data)
	}
}

type JSONFormatter struct{}

func (f *JSONFormatter) Format(data *ReportData) ([]byte, error) {
	output := map[string]interface{}{
		"mission_id":   data.MissionID,
		"targets":      data.Targets,
		"summary":      data.Summary,
		"findings":     data.Findings,
		"timeline":     data.Timeline,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
	return json.MarshalIndent(output, "", "  ")
}

type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Format(data *ReportData) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Mission Report: %s\n\n", data.MissionID))
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Count |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total Findings | %d |\n", data.Summary.TotalFindings))
	sb.WriteString(fmt.Sprintf("| Critical | %d |\n", data.Summary.CriticalCount))
	sb.WriteString(fmt.Sprintf("| High | %d |\n", data.Summary.HighCount))
	sb.WriteString(fmt.Sprintf("| Medium | %d |\n", data.Summary.MediumCount))
	sb.WriteString(fmt.Sprintf("| Low | %d |\n", data.Summary.LowCount))
	sb.WriteString(fmt.Sprintf("| Duration (s) | %d |\n", data.Summary.TotalDuration))
	sb.WriteString(fmt.Sprintf("| Success Rate | %.1f%% |\n\n", data.Summary.SuccessRate*100))

	sb.WriteString("## Targets\n\n")
	for _, t := range data.Targets {
		sb.WriteString(fmt.Sprintf("- %s\n", t))
	}
	sb.WriteString("\n")

	sb.WriteString("## Findings\n\n")
	for _, f := range data.Findings {
		sb.WriteString(fmt.Sprintf("### %s\n\n", f.Title))
		sb.WriteString(fmt.Sprintf("- **Severity:** %s\n", f.Severity))
		sb.WriteString(fmt.Sprintf("- **CVSS:** %.1f\n", f.CVSS))
		if f.CVE != "" {
			sb.WriteString(fmt.Sprintf("- **CVE:** %s\n", f.CVE))
		}
		if f.CWE != "" {
			sb.WriteString(fmt.Sprintf("- **CWE:** %s\n", f.CWE))
		}
		sb.WriteString(fmt.Sprintf("- **Confidence:** %s\n", f.Confidence))
		sb.WriteString(fmt.Sprintf("- **Status:** %s\n", f.Status))
		sb.WriteString(fmt.Sprintf("\n%s\n\n", f.Description))
	}

	sb.WriteString("## Timeline\n\n")
	for _, te := range data.Timeline {
		ts := time.Unix(te.Timestamp, 0).UTC().Format(time.RFC3339)
		sb.WriteString(fmt.Sprintf("- **%s** — %s: %s\n", ts, te.Event, te.Detail))
	}

	return []byte(sb.String()), nil
}

func buildSummary(findings []finding.Finding) ReportSummary {
	s := ReportSummary{TotalFindings: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case common.SeverityCritical:
			s.CriticalCount++
		case common.SeverityHigh:
			s.HighCount++
		case common.SeverityMedium:
			s.MediumCount++
		case common.SeverityLow:
			s.LowCount++
		}
	}
	if s.TotalFindings > 0 {
		confirmed := 0
		for _, f := range findings {
			if f.Status == finding.FindingStatusConfirmed {
				confirmed++
			}
		}
		s.SuccessRate = float64(confirmed) / float64(s.TotalFindings)
	}
	return s
}

func buildTimeline(findings []finding.Finding) []TimelineEntry {
	var timeline []TimelineEntry
	for _, f := range findings {
		entry := TimelineEntry{
			Timestamp: f.CreatedAt.Unix(),
			Event:     "finding_discovered",
			Detail:    fmt.Sprintf("%s (severity: %s)", f.Title, f.Severity),
		}
		timeline = append(timeline, entry)
	}
	return timeline
}

func mapToReportDTO(r *domainReport.Report) *dto.ReportResponse {
	return &dto.ReportResponse{
		ReportID:  r.ID.String(),
		MissionID: r.MissionID.String(),
		Title:     r.Title,
		Format:    string(r.Format),
		Data:      r.Content,
		Filename:  fmt.Sprintf("report-%s.%s", r.ID.String(), strings.ToLower(string(r.Format))),
		Summary: dto.ReportSummaryDTO{
			TotalFindings: r.Summary.TotalFindings,
			CriticalCount: r.Summary.CriticalCount,
			HighCount:     r.Summary.HighCount,
			MediumCount:   r.Summary.MediumCount,
			LowCount:      r.Summary.LowCount,
			TotalDuration: r.Summary.TotalDuration,
			SuccessRate:   r.Summary.SuccessRate,
		},
	}
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if v != "" && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
