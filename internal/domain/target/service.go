package target

import "context"

type ReconService interface {
	StartPassiveRecon(ctx context.Context, targetID string) error
	StartActiveRecon(ctx context.Context, targetID string, ports PortRange) error
	StartOSINTRecon(ctx context.Context, targetID string) error
	GetReconData(ctx context.Context, targetID string) (*Target, error)
	EnrichTarget(ctx context.Context, targetID string) error
}
