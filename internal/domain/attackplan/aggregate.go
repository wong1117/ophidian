package attackplan

import "github.com/ophidian/ophidian/internal/domain/common"

type AttackPlanAggregate struct {
	Plan    *AttackPlan
	Events  []DomainEvent
	Version int
}

func NewAttackPlanAggregate(plan *AttackPlan) *AttackPlanAggregate {
	return &AttackPlanAggregate{
		Plan:    plan,
		Events:  []DomainEvent{},
		Version: 0,
	}
}

func (a *AttackPlanAggregate) AddEvent(event DomainEvent) {
	a.Events = append(a.Events, event)
	a.Version++
}

func (a *AttackPlanAggregate) SelectPath(pathIndex int) error {
	if pathIndex < 0 || pathIndex >= len(a.Plan.RankedPaths) {
		return common.ErrInvalidTarget
	}
	a.Plan.Status = PlanActive
	a.Plan.UpdatedAt = common.Now()
	a.AddEvent(PathSelected{
		PlanID:    a.Plan.ID,
		PathIndex: pathIndex,
		Timestamp: common.Now(),
	})
	return nil
}
