package explainability

import (
	"context"
	"fmt"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	"github.com/stretchr/testify/assert"
)

type testRepo struct {
	explanations map[string]*Explanation
}

func newTestRepo() *testRepo {
	return &testRepo{explanations: make(map[string]*Explanation)}
}

func (r *testRepo) Save(ctx context.Context, e *Explanation) error {
	r.explanations[e.ID.String()] = e
	return nil
}

func (r *testRepo) FindByID(ctx context.Context, id string) (*Explanation, error) {
	e, ok := r.explanations[id]
	if !ok { return nil, fmt.Errorf("not found") }
	return e, nil
}

func (r *testRepo) FindByType(ctx context.Context, expType ExplanationType) ([]*Explanation, error) {
	var result []*Explanation
	for _, e := range r.explanations {
		if e.Type == expType { result = append(result, e) }
	}
	return result, nil
}

func (r *testRepo) FindByTarget(ctx context.Context, targetID string) ([]*Explanation, error) {
	var result []*Explanation
	for _, e := range r.explanations {
		if e.TargetID.String() == targetID { result = append(result, e) }
	}
	return result, nil
}

func (r *testRepo) FindAll(ctx context.Context) ([]*Explanation, error) {
	var result []*Explanation
	for _, e := range r.explanations { result = append(result, e) }
	return result, nil
}

func sampleFindings() []finding.Finding {
	return []finding.Finding{
		{ID: common.NewID(), Title: "SQL Injection", Severity: common.SeverityCritical, CVSS: 9.8, CVE: "CVE-2024-0001", Confidence: finding.ConfidenceConfirmed},
		{ID: common.NewID(), Title: "Outdated TLS", Severity: common.SeverityHigh, CVSS: 7.5, Confidence: finding.ConfidenceHigh},
		{ID: common.NewID(), Title: "Info Leak", Severity: common.SeverityLow, CVSS: 2.5, Confidence: finding.ConfidenceMedium},
	}
}

func TestExplainabilityService_RecordPlanGeneration(t *testing.T) {
	repo := newTestRepo()
	svc := NewExplainabilityService(repo)

	steps := []ReasoningStep{
		{Order: 1, Description: "Analyze available exploits", Inputs: []string{"service scan results"}, Output: "identified 3 matching exploits", Confidence: 0.9},
		{Order: 2, Description: "Rank exploits by success rate", Inputs: []string{"exploit database"}, Output: "selected CVE-2024-0001", Confidence: 0.85},
	}

	e, err := svc.RecordPlanGeneration(context.Background(), common.NewID(), "Generated attack plan for target web-server", steps, sampleFindings())

	assert.NoError(t, err)
	assert.Equal(t, TypePlanGeneration, e.Type)
	assert.NotEmpty(t, e.ReasoningChain)
	assert.NotEmpty(t, e.Evidence)
	assert.Greater(t, e.Confidence, 0.8)
	assert.NotEmpty(t, e.OutputDetail)
}

func TestExplainabilityService_RecordExploitSelection(t *testing.T) {
	repo := newTestRepo()
	svc := NewExplainabilityService(repo)

	steps := []ReasoningStep{
		{Order: 1, Description: "Check CVE match", Inputs: []string{"target services"}, Output: "CVE-2024-0001 matches Apache 2.4", Confidence: 0.95},
	}

	e, err := svc.RecordExploitSelection(context.Background(), common.NewID(), "Selected CVE-2024-0001 for Apache exploitation", steps, sampleFindings()[:1])

	assert.NoError(t, err)
	assert.Equal(t, TypeExploitSelection, e.Type)
}

func TestExplainabilityService_RecordRecommendation(t *testing.T) {
	repo := newTestRepo()
	svc := NewExplainabilityService(repo)

	steps := []ReasoningStep{
		{Order: 1, Description: "Evaluate risk", Inputs: []string{"findings"}, Output: "high risk identified", Confidence: 0.9},
	}

	e, err := svc.RecordRecommendation(context.Background(), common.NewID(), "Patch Apache to latest version", steps, sampleFindings())

	assert.NoError(t, err)
	assert.Equal(t, TypeRecommendation, e.Type)
}

func TestExplainabilityService_GetByType(t *testing.T) {
	repo := newTestRepo()
	svc := NewExplainabilityService(repo)

	svc.RecordPlanGeneration(context.Background(), common.NewID(), "Plan 1", []ReasoningStep{{Order: 1, Confidence: 0.9}}, nil)
	svc.RecordPlanGeneration(context.Background(), common.NewID(), "Plan 2", []ReasoningStep{{Order: 1, Confidence: 0.8}}, nil)
	svc.RecordExploitSelection(context.Background(), common.NewID(), "Exploit", []ReasoningStep{{Order: 1, Confidence: 0.7}}, nil)

	plans, err := svc.GetByType(context.Background(), TypePlanGeneration)
	assert.NoError(t, err)
	assert.Len(t, plans, 2)

	exploits, err := svc.GetByType(context.Background(), TypeExploitSelection)
	assert.NoError(t, err)
	assert.Len(t, exploits, 1)
}

func TestExplainabilityService_GetByTarget(t *testing.T) {
	repo := newTestRepo()
	svc := NewExplainabilityService(repo)

	targetID := common.NewID()
	svc.RecordPlanGeneration(context.Background(), targetID, "Plan A", []ReasoningStep{{Order: 1, Confidence: 0.9}}, nil)
	svc.RecordPlanGeneration(context.Background(), common.NewID(), "Plan B", []ReasoningStep{{Order: 1, Confidence: 0.8}}, nil)

	result, err := svc.GetByTarget(context.Background(), targetID.String())
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestExplainabilityService_GetHistory(t *testing.T) {
	repo := newTestRepo()
	svc := NewExplainabilityService(repo)

	svc.RecordPlanGeneration(context.Background(), common.NewID(), "P1", []ReasoningStep{{Order: 1, Confidence: 0.9}}, nil)
	svc.RecordExploitSelection(context.Background(), common.NewID(), "E1", []ReasoningStep{{Order: 1, Confidence: 0.8}}, nil)

	history, err := svc.GetHistory(context.Background())
	assert.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestCalculateConfidence(t *testing.T) {
	steps := []ReasoningStep{
		{Order: 1, Confidence: 1.0},
		{Order: 2, Confidence: 0.5},
		{Order: 3, Confidence: 0.0},
	}
	c := calculateConfidence(steps)
	assert.Greater(t, c, 0.3)
	assert.Less(t, c, 0.8)

	assert.Equal(t, 0.0, calculateConfidence(nil))
}

func TestCollectEvidence(t *testing.T) {
	findings := sampleFindings()
	evidence := collectEvidence(findings)

	assert.Len(t, evidence, 3)
	assert.Equal(t, "FINDING", evidence[0].Type)
	assert.Equal(t, "cve_database", evidence[0].Source)
	assert.Equal(t, "security_scan", evidence[1].Source)
}

func TestBuildDetail(t *testing.T) {
	steps := []ReasoningStep{
		{Order: 1, Description: "Step 1", Inputs: []string{"A", "B"}, Output: "Result 1", Confidence: 0.9},
	}
	detail := buildDetail("Test summary", steps, sampleFindings()[:1])

	assert.Contains(t, detail, "Test summary")
	assert.Contains(t, detail, "A, B")
	assert.Contains(t, detail, "Result 1")
	assert.Contains(t, detail, "CVE-2024-0001")
	assert.Contains(t, detail, "SQL Injection")
}
