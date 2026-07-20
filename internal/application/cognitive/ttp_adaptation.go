package cognitive

import (
	"context"
	"sort"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
)

type TTPAdaptationEngine struct {
	rag    *RAGMemory
	history []AdaptationEvent
}

type AdaptationEvent struct {
	PlanID       string
	StepIndex    int
	OriginalStep string
	NewStep      string
	Reason       string
	Success      bool
	Timestamp    common.UTCTime
}

func NewTTPAdaptationEngine(rag *RAGMemory) *TTPAdaptationEngine {
	return &TTPAdaptationEngine{
		rag:     rag,
		history: make([]AdaptationEvent, 0),
	}
}

func (e *TTPAdaptationEngine) Adapt(ctx context.Context, plan *attackplan.AttackPlan, feedback attackplan.StrategyFeedback) (*attackplan.Strategy, error) {
	strategy := &attackplan.Strategy{
		ID: common.NewID(),
	}

	for _, stepResult := range feedback.StepResults {
		if stepResult.Status == "FAILED" {
			altStep := e.findAlternative(ctx, stepResult.StepID, plan)
			if altStep != nil {
				strategy.Steps = append(strategy.Steps, *altStep)
				e.history = append(e.history, AdaptationEvent{
					PlanID:       plan.ID.String(),
					StepIndex:    len(strategy.Steps) - 1,
					OriginalStep: stepResult.StepID,
					NewStep:      altStep.Action,
					Reason:       "Step failed, using alternative technique",
				})
			}
		} else if stepResult.Status == "SUCCESS" {
			e.history = append(e.history, AdaptationEvent{
				PlanID:       plan.ID.String(),
				StepIndex:    len(strategy.Steps),
				OriginalStep: stepResult.StepID,
				Success:      true,
				Timestamp:    common.Now(),
			})
		}
	}

	strategy.Name = "Adapted Strategy"
	strategy.Description = fmt.Sprintf("Auto-adapted from plan %s based on %d feedback results", plan.ID, len(feedback.StepResults))

	return strategy, nil
}

func (e *TTPAdaptationEngine) findAlternative(ctx context.Context, stepID string, plan *attackplan.AttackPlan) *attackplan.AttackStep {
	for _, node := range plan.Graph.Nodes {
		if node.ID == stepID {
			return &attackplan.AttackStep{
				Action:     node.Type,
				Service:    node.Service,
				CVE:        node.CVE,
				Confidence: node.Confidence * 0.8,
				RiskLevel:  node.RiskLevel,
			}
		}
	}
	return nil
}

func (e *TTPAdaptationEngine) ReorderWorkflow(ctx context.Context, plan *attackplan.AttackPlan, results []attackplan.StepResult) []attackplan.RankedPath {
	if len(results) == 0 {
		return plan.RankedPaths
	}

	successMap := make(map[string]bool)
	for _, r := range results {
		successMap[r.StepID] = r.Status == "SUCCESS"
	}

	reordered := make([]attackplan.RankedPath, 0)
	for _, path := range plan.RankedPaths {
		failedBefore := false
		adjustedPath := path
		adjustedPath.Confidence = path.Confidence

		for _, nodeID := range path.Nodes {
			if success, ok := successMap[nodeID]; ok && !success {
				failedBefore = true
				adjustedPath.Confidence *= 0.5
				adjustedPath.RiskLevel = common.RiskHigh
				break
			}
			if failedBefore {
				break
			}
		}

		if !failedBefore {
			adjustedPath.Confidence *= 1.1
		}

		reordered = append(reordered, adjustedPath)
	}

	sort.Slice(reordered, func(i, j int) bool {
		return reordered[i].Confidence > reordered[j].Confidence
	})

	return reordered
}
