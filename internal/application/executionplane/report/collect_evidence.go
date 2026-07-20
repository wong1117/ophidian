package report

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type EvidenceCollector interface {
	Collect(ctx context.Context, findingID string) ([]finding.Evidence, error)
}

type CollectEvidenceUseCase struct {
	findingRepo finding.FindingRepository
	collector   EvidenceCollector
}

func NewCollectEvidenceUseCase(fr finding.FindingRepository, c EvidenceCollector) *CollectEvidenceUseCase {
	return &CollectEvidenceUseCase{findingRepo: fr, collector: c}
}

func (uc *CollectEvidenceUseCase) Execute(ctx context.Context, findingID string) ([]finding.Evidence, error) {
	f, err := uc.findingRepo.FindByID(ctx, findingID)
	if err != nil {
		return nil, err
	}

	evidence, err := uc.collector.Collect(ctx, findingID)
	if err != nil {
		return nil, err
	}

	for _, ev := range evidence {
		if err := uc.findingRepo.SaveEvidence(ctx, &ev); err != nil {
			return nil, err
		}
	}

	return evidence, nil
}
