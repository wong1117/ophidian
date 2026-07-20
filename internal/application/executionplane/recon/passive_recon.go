package recon

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/target"
)

type PassiveReconUseCase struct {
	targetRepo target.TargetRepository
	scanner    Scanner
}

type Scanner interface {
	ScanPorts(ctx context.Context, ip string, ports target.PortRange) ([]target.Service, error)
	ResolveDNS(ctx context.Context, domain string) ([]string, error)
	QueryWhois(ctx context.Context, domain string) (map[string]string, error)
}

func NewPassiveReconUseCase(tr target.TargetRepository, s Scanner) *PassiveReconUseCase {
	return &PassiveReconUseCase{targetRepo: tr, scanner: s}
}

func (uc *PassiveReconUseCase) Execute(ctx context.Context, targetID string) error {
	t, err := uc.targetRepo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}

	for _, domain := range t.Domains {
		ips, err := uc.scanner.ResolveDNS(ctx, domain.Name)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			t.IPs = append(t.IPs, target.IP{Address: ip, Type: target.IPv4})
		}
	}

	return uc.targetRepo.Update(ctx, t)
}
