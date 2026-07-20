package recon

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/target"
)

type ActiveReconUseCase struct {
	targetRepo target.TargetRepository
	scanner    Scanner
}

func NewActiveReconUseCase(tr target.TargetRepository, s Scanner) *ActiveReconUseCase {
	return &ActiveReconUseCase{targetRepo: tr, scanner: s}
}

func (uc *ActiveReconUseCase) Execute(ctx context.Context, targetID string, ports target.PortRange) error {
	t, err := uc.targetRepo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}

	for _, ip := range t.IPs {
		services, err := uc.scanner.ScanPorts(ctx, ip.Address, ports)
		if err != nil {
			continue
		}
		t.Services = append(t.Services, services...)
	}

	return uc.targetRepo.Update(ctx, t)
}
