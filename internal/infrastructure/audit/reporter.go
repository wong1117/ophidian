package audit

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

type ReportGenerator struct {
	correlator *EventCorrelator
	replay     *ReplayEngine
	upcaster   *UpcasterChain
}

func NewReportGenerator(correlator *EventCorrelator, replay *ReplayEngine, upcaster *UpcasterChain) *ReportGenerator {
	return &ReportGenerator{correlator: correlator, replay: replay, upcaster: upcaster}
}

type AuditReport struct {
	GeneratedAt  time.Time
	MissionID    string
	TotalEvents  int
	Duration     time.Duration
	Phases       []PhaseSection
	KillChain    *KillChainVisualData
	Findings     []FindingSummary
	Evidence     []EvidenceItem
}

type PhaseSection struct {
	Name        string
	EventCount  int
	Events      []EventSummary
}

type EventSummary struct {
	ID        string
	Type      string
	Timestamp time.Time
	Summary   string
}

type FindingSummary struct {
	Title       string
	Severity    string
	Description string
	CVSS        float64
}

type EvidenceItem struct {
	ID          string
	Type        string
	Source      string
	Timestamp   time.Time
	Description string
}

func (r *ReportGenerator) GenerateReport(ctx context.Context, missionID string, from, to time.Time) (*AuditReport, error) {
	result, err := r.correlator.Correlate(ctx, missionID, from, to)
	if err != nil {
		return nil, fmt.Errorf("generate report: %w", err)
	}

	killChain, err := r.correlator.BuildKillChainVisual(ctx, missionID, from, to)
	if err != nil {
		return nil, fmt.Errorf("generate report killchain: %w", err)
	}

	report := &AuditReport{
		GeneratedAt: time.Now(),
		MissionID:   missionID,
		TotalEvents: result.TotalEvents,
		Duration:    result.Duration,
		KillChain:   killChain,
	}

	for _, phaseName := range phaseOrder {
		if phase, ok := result.Phases[phaseName]; ok {
			section := PhaseSection{
				Name:       phaseName,
				EventCount: len(phase.Events),
			}
			for _, ce := range phase.Events {
				section.Events = append(section.Events, EventSummary{
					ID:        ce.Event.Meta.ID,
					Type:      ce.Event.Meta.EventType,
					Timestamp: ce.Event.Meta.OccurredAt,
					Summary:   extractSummary(ce.Event),
				})
			}
			report.Phases = append(report.Phases, section)
		}
	}

	return report, nil
}

func (r *ReportGenerator) ExportMarkdown(report *AuditReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Ophidian Mission Audit Report\n\n"))
	sb.WriteString(fmt.Sprintf("**Mission:** %s\n", report.MissionID))
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", report.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Duration:** %s\n", report.Duration.String()))
	sb.WriteString(fmt.Sprintf("**Events:** %d\n\n", report.TotalEvents))

	sb.WriteString("## Kill Chain Summary\n\n")
	if report.KillChain != nil {
		for _, phase := range report.KillChain.Phases {
			if phase.EventCount > 0 {
				sb.WriteString(fmt.Sprintf("- **%s**: %d events (%s → %s)\n",
					phase.Name, phase.EventCount, phase.StartTime, phase.EndTime))
			}
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Execution Timeline\n\n")
	for _, section := range report.Phases {
		sb.WriteString(fmt.Sprintf("### %s\n\n", section.Name))
		for _, evt := range section.Events {
			sb.WriteString(fmt.Sprintf("- `%s` **%s** - %s\n",
				evt.Timestamp.Format("15:04:05"), evt.Type, evt.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Findings\n\n")
	for _, f := range report.Findings {
		sb.WriteString(fmt.Sprintf("- **[%s]** %s (CVSS: %.1f)\n", f.Severity, f.Title, f.CVSS))
		if f.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", f.Description))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Evidence\n\n")
	for _, ev := range report.Evidence {
		sb.WriteString(fmt.Sprintf("- `%s` [%s] %s (%s)\n",
			ev.Timestamp.Format("15:04:05"), ev.Type, ev.Description, ev.Source))
	}

	return sb.String()
}

func (r *ReportGenerator) ExportBurpXML(report *AuditReport) string {
	type BurpItem struct {
		Name        string        `xml:"name"`
		Host        string        `xml:"host"`
		Port        string        `xml:"port"`
		Protocol    string        `xml:"protocol"`
		Severity    string        `xml:"severity"`
		Confidence  string        `xml:"confidence"`
		Background  string        `xml:"issueBackground"`
	}

	type BurpReport struct {
		XMLName xml.Name   `xml:"issues"`
		Items   []BurpItem `xml:"issue"`
	}

	items := make([]BurpItem, 0)
	for _, f := range report.Findings {
		items = append(items, BurpItem{
			Name:       f.Title,
			Severity:   f.Severity,
			Confidence: "Certain",
			Background: f.Description,
		})
	}

	data, _ := xml.MarshalIndent(BurpReport{Items: items}, "", "  ")
	return xml.Header + string(data)
}

func (r *ReportGenerator) ExportJSON(report *AuditReport) string {
	data, _ := json.MarshalIndent(report, "", "  ")
	return string(data)
}

func extractSummary(evt Envelope) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return evt.Meta.EventType
	}
	if content, ok := payload["description"].(string); ok && content != "" {
		return content
	}
	if content, ok := payload["title"].(string); ok && content != "" {
		return content
	}
	if content, ok := payload["summary"].(string); ok && content != "" {
		return content
	}
	return evt.Meta.EventType
}
