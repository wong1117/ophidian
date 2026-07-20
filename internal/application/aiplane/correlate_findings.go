package aiplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type CorrelateFindingsUseCase struct {
	findingRepo finding.FindingRepository
	planner     attackplan.AIPlanner
}

func NewCorrelateFindingsUseCase(fr finding.FindingRepository, planner attackplan.AIPlanner) *CorrelateFindingsUseCase {
	return &CorrelateFindingsUseCase{findingRepo: fr, planner: planner}
}

func (uc *CorrelateFindingsUseCase) Execute(ctx context.Context, missionID string) (*attackplan.CorrelationResult, error) {
	findings, err := uc.findingRepo.FindByMission(ctx, missionID)
	if err != nil {
		return nil, err
	}

	correlation, err := uc.planner.CorrelateFindings(ctx, findings)
	if err != nil {
		return nil, err
	}

	return correlation, nil
}
