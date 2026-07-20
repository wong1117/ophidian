package aiplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
)

type RankPathsUseCase struct {
	planner attackplan.AIPlanner
}

func NewRankPathsUseCase(planner attackplan.AIPlanner) *RankPathsUseCase {
	return &RankPathsUseCase{planner: planner}
}

func (uc *RankPathsUseCase) Execute(ctx context.Context, graph attackplan.AttackGraph) ([]attackplan.RankedPath, error) {
	return uc.planner.RankPaths(ctx, graph)
}
