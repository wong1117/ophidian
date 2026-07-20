package target

import "context"

type TargetRepository interface {
	Save(ctx context.Context, target *Target) error
	FindByID(ctx context.Context, id string) (*Target, error)
	FindByIP(ctx context.Context, ip string) (*Target, error)
	FindByDomain(ctx context.Context, domain string) (*Target, error)
	FindAll(ctx context.Context, filter TargetFilter) ([]*Target, error)
	Update(ctx context.Context, target *Target) error
	Delete(ctx context.Context, id string) error
}

type TargetFilter struct {
	Tags   []string
	Limit  int
	Offset int
}
