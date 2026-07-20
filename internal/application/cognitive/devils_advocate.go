package cognitive

import (
	"context"
	"fmt"
	"math"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
)

type ReviewResult struct {
	PlanID        string
	Passed        bool
	RiskScore     float64
	FailChance    float64
	Weaknesses    []string
	Risks         []RiskItem
	Alternatives  []AlternativeStrategy
	StealthScore  float64
	FinalScore    float64
	Recommendation string
}

type RiskItem struct {
	Category    string
	Description string
	Severity    string
	Mitigation  string
}

type AlternativeStrategy struct {
	Description string
	Confidence  float64
	Stealth     float64
	RiskLevel   common.RiskLevel
}

type DevilsAdvocate struct {
	rag      *RAGMemory
	minScore float64
}

func NewDevilsAdvocate(rag *RAGMemory) *DevilsAdvocate {
	return &DevilsAdvocate{
		rag:      rag,
		minScore: 0.3,
	}
}

func (da *DevilsAdvocate) Review(ctx context.Context, plan *attackplan.AttackPlan) (*ReviewResult, error) {
	result := &ReviewResult{
		PlanID:       plan.ID.String(),
		Passed:       true,
		Weaknesses:   make([]string, 0),
		Risks:        make([]RiskItem, 0),
		Alternatives: make([]AlternativeStrategy, 0),
	}

	if len(plan.RankedPaths) == 0 {
		result.Weaknesses = append(result.Weaknesses, "No attack paths defined")
		result.Passed = false
		return result, nil
	}

	bestPath := plan.RankedPaths[0]
	riskCount := 0
	for _, path := range plan.RankedPaths {
		if path.RiskLevel == common.RiskCritical || path.RiskLevel == common.RiskHigh {
			riskCount++
		}
	}

	if riskCount > len(plan.RankedPaths)/2 {
		result.Risks = append(result.Risks, RiskItem{
			Category:    "Risk Distribution",
			Description: fmt.Sprintf("%d of %d paths have high/critical risk", riskCount, len(plan.RankedPaths)),
			Severity:    "HIGH",
			Mitigation:  "Consider alternative approaches with lower risk profiles",
		})
		result.Weaknesses = append(result.Weaknesses, "Majority of paths have unacceptable risk")
	}

	if bestPath.Confidence < 0.5 {
		result.Weaknesses = append(result.Weaknesses, fmt.Sprintf("Best path confidence too low: %.2f", bestPath.Confidence))
		result.Risks = append(result.Risks, RiskItem{
			Category:    "Confidence",
			Description: fmt.Sprintf("Path confidence %.2f is below acceptable threshold", bestPath.Confidence),
			Severity:    "MEDIUM",
			Mitigation:  "Gather more recon data to increase confidence",
		})
	}

	if plan.Confidence < 0.4 {
		result.Weaknesses = append(result.Weaknesses, fmt.Sprintf("Overall plan confidence too low: %.2f", plan.Confidence))
	}

	result.RiskScore = float64(bestPath.RiskLevel)
	result.FailChance = 1.0 - bestPath.Confidence

	result.StealthScore = 1.0 - math.Max(0, math.Min(1, result.RiskScore))
	result.FinalScore = (bestPath.Confidence*0.4 + result.StealthScore*0.3 + (1-result.FailChance)*0.3)
	result.FailChance = bestPath.Confidence

	da.generateAlternatives(ctx, plan, result)

	if result.FinalScore < da.minScore {
		result.Passed = false
		result.Recommendation = "REJECTED: Plan does not meet minimum quality threshold"
	} else if result.FinalScore < 0.6 {
		result.Recommendation = "MODIFIED: Plan needs improvements before approval"
	} else {
		result.Recommendation = "ACCEPTED: Plan meets quality standards"
	}

	return result, nil
}

func (da *DevilsAdvocate) generateAlternatives(ctx context.Context, plan *attackplan.AttackPlan, result *ReviewResult) {
	for i, path := range plan.RankedPaths {
		if i == 0 {
			continue
		}
		if len(result.Alternatives) >= 3 {
			break
		}
		result.Alternatives = append(result.Alternatives, AlternativeStrategy{
			Description: fmt.Sprintf("Alternative path %d with risk level %s", i+1, path.RiskLevel),
			Confidence:  path.Confidence,
			Stealth:     1.0 - float64(path.RiskLevel),
			RiskLevel:   path.RiskLevel,
		})
	}
}
