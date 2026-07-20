package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/target"
)

type TargetRepository struct {
	pool *pgxpool.Pool
}

func NewTargetRepository(pool *pgxpool.Pool) *TargetRepository {
	return &TargetRepository{pool: pool}
}

func (r *TargetRepository) Save(ctx context.Context, t *target.Target) error {
	return nil
}

func (r *TargetRepository) FindByID(ctx context.Context, id string) (*target.Target, error) {
	return nil, nil
}

func (r *TargetRepository) FindByIP(ctx context.Context, ip string) (*target.Target, error) {
	return nil, nil
}

func (r *TargetRepository) FindByDomain(ctx context.Context, domain string) (*target.Target, error) {
	return nil, nil
}

func (r *TargetRepository) FindAll(ctx context.Context, filter target.TargetFilter) ([]*target.Target, error) {
	return nil, nil
}

func (r *TargetRepository) Update(ctx context.Context, t *target.Target) error {
	return nil
}

func (r *TargetRepository) Delete(ctx context.Context, id string) error {
	return nil
}
