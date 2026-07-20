package aiplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
)

type EvaluateConfidenceUseCase struct {
	planner attackplan.AIPlanner
}

func NewEvaluateConfidenceUseCase(planner attackplan.AIPlanner) *EvaluateConfidenceUseCase {
	return &EvaluateConfidenceUseCase{planner: planner}
}

func (uc *EvaluateConfidenceUseCase) Execute(ctx context.Context, plan *attackplan.AttackPlan, evidence []attackplan.Evidence) (float64, error) {
	return uc.planner.EvaluateConfidence(ctx, plan, evidence)
}
