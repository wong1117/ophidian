package attackplan

import "context"

type AttackPlanRepository interface {
	Save(ctx context.Context, plan *AttackPlan) error
	FindByID(ctx context.Context, id string) (*AttackPlan, error)
	FindByMission(ctx context.Context, missionID string) ([]*AttackPlan, error)
	Update(ctx context.Context, plan *AttackPlan) error
	Delete(ctx context.Context, id string) error
}
