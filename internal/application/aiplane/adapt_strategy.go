package aiplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
)

type AdaptStrategyUseCase struct {
	planner attackplan.AIPlanner
}

func NewAdaptStrategyUseCase(planner attackplan.AIPlanner) *AdaptStrategyUseCase {
	return &AdaptStrategyUseCase{planner: planner}
}

func (uc *AdaptStrategyUseCase) Execute(ctx context.Context, planID string, feedback attackplan.StrategyFeedback) (*attackplan.Strategy, error) {
	return uc.planner.AdaptStrategy(ctx, planID, feedback)
}
