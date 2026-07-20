package recon

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/target"
)

type OSINTProvider interface {
	SearchDomain(ctx context.Context, domain string) ([]string, error)
	SearchIP(ctx context.Context, ip string) (map[string]interface{}, error)
	SearchCertificates(ctx context.Context, domain string) ([]string, error)
}

type OSINTReconUseCase struct {
	targetRepo target.TargetRepository
	providers  []OSINTProvider
}

func NewOSINTReconUseCase(tr target.TargetRepository, providers []OSINTProvider) *OSINTReconUseCase {
	return &OSINTReconUseCase{targetRepo: tr, providers: providers}
}

func (uc *OSINTReconUseCase) Execute(ctx context.Context, targetID string) error {
	t, err := uc.targetRepo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}

	for _, domain := range t.Domains {
		for _, provider := range uc.providers {
			subdomains, err := provider.SearchDomain(ctx, domain.Name)
			if err != nil {
				continue
			}
			for _, sd := range subdomains {
				t.Hostnames = append(t.Hostnames, sd)
			}
		}
	}

	return uc.targetRepo.Update(ctx, t)
}
