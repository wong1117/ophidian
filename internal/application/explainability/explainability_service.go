package explainability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type ExplanationType string

const (
	TypePlanGeneration     ExplanationType = "PLAN_GENERATION"
	TypeExploitSelection   ExplanationType = "EXPLOIT_SELECTION"
	TypeRecommendation     ExplanationType = "RECOMMENDATION"
	TypeRiskAssessment     ExplanationType = "RISK_ASSESSMENT"
)

type EvidenceRef struct {
	ID          common.ID
	Type        string
	Description string
	Source      string
	Confidence  float64
	Impact      float64
}

type ReasoningStep struct {
	Order       int
	Description string
	Inputs      []string
	Output      string
	Confidence  float64
}

type Explanation struct {
	ID              common.ID
	Type            ExplanationType
	TargetID        common.ID
	OutputSummary   string
	OutputDetail    string
	Confidence      float64
	ReasoningChain  []ReasoningStep
	Evidence        []EvidenceRef
	SourceFindings  []finding.Finding
	Metadata        map[string]interface{}
	CreatedAt       time.Time
}

type Repository interface {
	Save(ctx context.Context, e *Explanation) error
	FindByID(ctx context.Context, id string) (*Explanation, error)
	FindByType(ctx context.Context, expType ExplanationType) ([]*Explanation, error)
	FindByTarget(ctx context.Context, targetID string) ([]*Explanation, error)
	FindAll(ctx context.Context) ([]*Explanation, error)
}

type ExplainabilityService struct {
	repo Repository
}

func NewExplainabilityService(repo Repository) *ExplainabilityService {
	return &ExplainabilityService{repo: repo}
}

func (s *ExplainabilityService) RecordPlanGeneration(ctx context.Context, targetID common.ID, planSummary string, steps []ReasoningStep, findings []finding.Finding) (*Explanation, error) {
	return s.record(ctx, TypePlanGeneration, targetID, planSummary, steps, findings)
}

func (s *ExplainabilityService) RecordExploitSelection(ctx context.Context, targetID common.ID, exploitSummary string, steps []ReasoningStep, findings []finding.Finding) (*Explanation, error) {
	return s.record(ctx, TypeExploitSelection, targetID, exploitSummary, steps, findings)
}

func (s *ExplainabilityService) RecordRecommendation(ctx context.Context, targetID common.ID, recSummary string, steps []ReasoningStep, findings []finding.Finding) (*Explanation, error) {
	return s.record(ctx, TypeRecommendation, targetID, recSummary, steps, findings)
}

func (s *ExplainabilityService) RecordRiskAssessment(ctx context.Context, targetID common.ID, riskSummary string, steps []ReasoningStep, findings []finding.Finding) (*Explanation, error) {
	return s.record(ctx, TypeRiskAssessment, targetID, riskSummary, steps, findings)
}

func (s *ExplainabilityService) record(ctx context.Context, expType ExplanationType, targetID common.ID, summary string, steps []ReasoningStep, findings []finding.Finding) (*Explanation, error) {
	confidence := calculateConfidence(steps)
	evidence := collectEvidence(findings)

	e := &Explanation{
		ID:             common.NewID(),
		Type:           expType,
		TargetID:       targetID,
		OutputSummary:  summary,
		OutputDetail:   buildDetail(summary, steps, findings),
		Confidence:     confidence,
		ReasoningChain: steps,
		Evidence:       evidence,
		SourceFindings: findings,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.Save(ctx, e); err != nil {
		return nil, fmt.Errorf("record explanation: %w", err)
	}

	return e, nil
}

func (s *ExplainabilityService) GetByID(ctx context.Context, id string) (*Explanation, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *ExplainabilityService) GetByType(ctx context.Context, expType ExplanationType) ([]*Explanation, error) {
	return s.repo.FindByType(ctx, expType)
}

func (s *ExplainabilityService) GetByTarget(ctx context.Context, targetID string) ([]*Explanation, error) {
	return s.repo.FindByTarget(ctx, targetID)
}

func (s *ExplainabilityService) GetHistory(ctx context.Context) ([]*Explanation, error) {
	return s.repo.FindAll(ctx)
}

func calculateConfidence(steps []ReasoningStep) float64 {
	if len(steps) == 0 {
		return 0
	}
	var total float64
	for _, s := range steps {
		total += s.Confidence
	}
	avg := total / float64(len(steps))

	var weightedSum, weightSum float64
	for i, s := range steps {
		weight := float64(len(steps) - i)
		weightedSum += s.Confidence * weight
		weightSum += weight
	}
	weightedAvg := weightedSum / weightSum

	return (avg + weightedAvg) / 2.0
}

func collectEvidence(findings []finding.Finding) []EvidenceRef {
	var evidence []EvidenceRef
	for _, f := range findings {
		ev := EvidenceRef{
			ID:          f.ID,
			Type:        "FINDING",
			Description: f.Title,
			Source:      "security_scan",
			Confidence:  confidenceToFloat(f.Confidence),
			Impact:      f.CVSS,
		}
		if f.CVE != "" {
			ev.Source = "cve_database"
		}
		evidence = append(evidence, ev)
	}
	return evidence
}

func buildDetail(summary string, steps []ReasoningStep, findings []finding.Finding) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Explanation: %s\n\n", summary))
	sb.WriteString("Reasoning chain:\n")
	for _, step := range steps {
		sb.WriteString(fmt.Sprintf("  %d. %s → %s (confidence: %.0f%%)\n",
			step.Order, strings.Join(step.Inputs, ", "), step.Output, step.Confidence*100))
	}
	sb.WriteString(fmt.Sprintf("\nBased on %d findings:\n", len(findings)))
	for _, f := range findings {
		sb.WriteString(fmt.Sprintf("  - [%s] %s (CVSS: %.1f, Confidence: %s)\n",
			f.Severity, f.Title, f.CVSS, f.Confidence))
		if f.CVE != "" {
			sb.WriteString(fmt.Sprintf("    CVE: %s\n", f.CVE))
		}
	}
	return sb.String()
}

func confidenceToFloat(c finding.ConfidenceLevel) float64 {
	switch c {
	case finding.ConfidenceConfirmed: return 1.0
	case finding.ConfidenceHigh: return 0.8
	case finding.ConfidenceMedium: return 0.5
	default: return 0.3
	}
}
