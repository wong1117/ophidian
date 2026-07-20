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
	domainFindings, err := uc.findingRepo.FindByMission(ctx, missionID)
	if err != nil {
		return nil, err
	}

	planFindings := make([]attackplan.Finding, len(domainFindings))
	for i, f := range domainFindings {
		planFindings[i] = attackplan.Finding{
			ID:          f.ID.String(),
			Title:       f.Title,
			Description: f.Description,
			Severity:    string(f.Severity),
			CVE:         f.CVE,
			Confidence:  string(f.Confidence),
		}
	}

	correlation, err := uc.planner.CorrelateFindings(ctx, planFindings)
	if err != nil {
		return nil, err
	}

	return correlation, nil
}
