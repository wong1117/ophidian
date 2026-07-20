package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
)

type AttackPlanRepository struct {
	pool *pgxpool.Pool
}

func NewAttackPlanRepository(pool *pgxpool.Pool) *AttackPlanRepository {
	return &AttackPlanRepository{pool: pool}
}

func (r *AttackPlanRepository) Save(ctx context.Context, p *attackplan.AttackPlan) error {
	return nil
}

func (r *AttackPlanRepository) FindByID(ctx context.Context, id string) (*attackplan.AttackPlan, error) {
	return nil, nil
}

func (r *AttackPlanRepository) FindByMission(ctx context.Context, missionID string) ([]*attackplan.AttackPlan, error) {
	return nil, nil
}

func (r *AttackPlanRepository) Update(ctx context.Context, p *attackplan.AttackPlan) error {
	return nil
}

func (r *AttackPlanRepository) Delete(ctx context.Context, id string) error {
	return nil
}
