package recommendation

import (
	"context"
	"fmt"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	"github.com/stretchr/testify/assert"
)

type testRepo struct {
	recs map[string]*Recommendation
}

func newTestRepo() *testRepo {
	return &testRepo{recs: make(map[string]*Recommendation)}
}

func (r *testRepo) Save(ctx context.Context, rec *Recommendation) error {
	r.recs[rec.ID.String()] = rec
	return nil
}

func (r *testRepo) FindByID(ctx context.Context, id string) (*Recommendation, error) {
	rec, ok := r.recs[id]
	if !ok { return nil, fmt.Errorf("not found") }
	return rec, nil
}

func (r *testRepo) FindByCategory(ctx context.Context, category Category) ([]*Recommendation, error) {
	var result []*Recommendation
	for _, rec := range r.recs {
		if rec.Category == category {
			result = append(result, rec)
		}
	}
	return result, nil
}

func (r *testRepo) FindAll(ctx context.Context) ([]*Recommendation, error) {
	var result []*Recommendation
	for _, rec := range r.recs {
		result = append(result, rec)
	}
	return result, nil
}

func (r *testRepo) Delete(ctx context.Context, id string) error {
	delete(r.recs, id)
	return nil
}

func TestRecommendationService_Generate(t *testing.T) {
	repo := newTestRepo()
	svc := NewRecommendationService(repo)

	input := &AssessmentInput{
		Findings: []finding.Finding{
			{
				ID:          common.NewID(),
				Title:       "SQL Injection",
				Description: "Found SQL injection in login page",
				Severity:    common.SeverityCritical,
				CVSS:        9.8,
				CVE:         "CVE-2024-0001",
				CWE:         "CWE-89",
				Confidence:  finding.ConfidenceConfirmed,
			},
			{
				ID:          common.NewID(),
				Title:       "Outdated TLS",
				Description: "Server uses TLS 1.0",
				Severity:    common.SeverityHigh,
				CVSS:        7.5,
				Confidence:  finding.ConfidenceHigh,
			},
			{
				ID:          common.NewID(),
				Title:       "Info disclosure",
				Description: "Server header leak",
				Severity:    common.SeverityLow,
				CVSS:        2.0,
				Confidence:  finding.ConfidenceMedium,
			},
		},
		Environment:     "production",
		AssetCriticality: 3,
		ComplianceReqs:  []string{"PCI-DSS", "SOC2"},
	}

	result, err := svc.Generate(context.Background(), input)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Greater(t, result[0].Priority, result[len(result)-1].Priority, "first should have highest priority")
}

func TestRecommendationService_Generate_Empty(t *testing.T) {
	repo := newTestRepo()
	svc := NewRecommendationService(repo)

	input := &AssessmentInput{
		Findings: []finding.Finding{},
	}

	result, err := svc.Generate(context.Background(), input)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestRecommendationService_Ranking(t *testing.T) {
	repo := newTestRepo()
	svc := NewRecommendationService(repo)

	input := &AssessmentInput{
		Findings: []finding.Finding{
			{ID: common.NewID(), Title: "Low", Severity: common.SeverityLow, CVSS: 1.0, Confidence: finding.ConfidenceLow},
			{ID: common.NewID(), Title: "Critical", Severity: common.SeverityCritical, CVSS: 10.0, Confidence: finding.ConfidenceConfirmed},
		},
	}

	result, _ := svc.Generate(context.Background(), input)
	assert.NotEmpty(t, result)
	assert.Greater(t, result[0].Priority, result[len(result)-1].Priority)
}

func TestRecommendationService_GetHistory(t *testing.T) {
	repo := newTestRepo()
	svc := NewRecommendationService(repo)

	input := &AssessmentInput{
		Findings: []finding.Finding{
			{ID: common.NewID(), Title: "Test", Severity: common.SeverityHigh, CVSS: 7.0, Confidence: finding.ConfidenceHigh},
		},
	}

	svc.Generate(context.Background(), input)

	history, err := svc.GetHistory(context.Background(), "")
	assert.NoError(t, err)
	assert.NotEmpty(t, history)

	secHistory, err := svc.GetHistory(context.Background(), CategorySecurity)
	assert.NoError(t, err)
	assert.NotEmpty(t, secHistory)
}

func TestRecommendationService_CustomRules(t *testing.T) {
	repo := newTestRepo()
	svc := NewRecommendationService(repo)

	svc.SetRules(map[Category]ScoringRule{
		CategoryCritical: {CategoryCritical, 10.0, 5.0, 2.0, 1.0, 20.0},
	})

	input := &AssessmentInput{
		Findings: []finding.Finding{
			{ID: common.NewID(), Title: "Critical Vuln", Severity: common.SeverityCritical, CVSS: 9.5, Confidence: finding.ConfidenceConfirmed},
		},
	}

	result, _ := svc.Generate(context.Background(), input)
	assert.NotEmpty(t, result)
	assert.Greater(t, result[0].Priority, 50)
}

func TestScoringDefaults(t *testing.T) {
	r := DefaultScoringRule(CategoryCritical)
	assert.Equal(t, 3.0, r.SeverityWeight)
	assert.Equal(t, 2.0, r.CVSSWeight)

	c := DefaultScoringRule(CategoryBestPractice)
	assert.Equal(t, 1.5, c.SeverityWeight)
	assert.Equal(t, 1.0, c.CVSSWeight)
	assert.Equal(t, 0.0, c.ComplianceBonus)
}

func TestConfidenceCalculation(t *testing.T) {
	svc := NewRecommendationService(nil)
	rule := DefaultScoringRule(CategoryCritical)

	c := svc.calculateConfidence(finding.Finding{CVSS: 10.0, Confidence: finding.ConfidenceConfirmed}, rule)
	assert.InDelta(t, 1.0, c, 0.01)

	c = svc.calculateConfidence(finding.Finding{CVSS: 0, Confidence: finding.ConfidenceLow}, rule)
	assert.InDelta(t, 0.56, c, 0.01)
}

func TestScoreCalculation(t *testing.T) {
	svc := NewRecommendationService(nil)
	rule := DefaultScoringRule(CategoryCritical)

	input := &AssessmentInput{
		AssetCriticality: 3,
		ComplianceReqs:   []string{"PCI"},
	}

	score := svc.calculateScore(finding.Finding{
		Severity:   common.SeverityHigh,
		CVSS:       8.0,
		Confidence: finding.ConfidenceConfirmed,
	}, input, rule)

	assert.Greater(t, score, 20.0)
}

func TestHelperFunctions(t *testing.T) {
	assert.Equal(t, 5, sevToInt(common.SeverityCritical))
	assert.Equal(t, 4, sevToInt(common.SeverityHigh))
	assert.Equal(t, 3, sevToInt(common.SeverityMedium))
	assert.Equal(t, 2, sevToInt(common.SeverityLow))
	assert.Equal(t, 1, sevToInt(common.SeverityInfo))

	assert.Equal(t, 1.0, confidenceToFloat(finding.ConfidenceConfirmed))
	assert.Equal(t, 0.8, confidenceToFloat(finding.ConfidenceHigh))
	assert.Equal(t, 0.5, confidenceToFloat(finding.ConfidenceMedium))
	assert.Equal(t, 0.3, confidenceToFloat(finding.ConfidenceLow))

	assert.Equal(t, 0.5, clamp(0.5, 0, 1))
	assert.Equal(t, 0.0, clamp(-1, 0, 1))
	assert.Equal(t, 1.0, clamp(2, 0, 1))
}
