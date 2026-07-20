package target

import "github.com/ophidian/ophidian/internal/domain/common"

type TargetAggregate struct {
	Target  *Target
	Events  []DomainEvent
	Version int
}

func NewTargetAggregate(target *Target) *TargetAggregate {
	return &TargetAggregate{
		Target:  target,
		Events:  []DomainEvent{},
		Version: 0,
	}
}

func (a *TargetAggregate) AddEvent(event DomainEvent) {
	a.Events = append(a.Events, event)
	a.Version++
}

func (a *TargetAggregate) AddService(svc Service) {
	a.Target.Services = append(a.Target.Services, svc)
	a.Target.UpdatedAt = common.Now()
	a.AddEvent(ServiceDetected{
		TargetID:  a.Target.ID,
		Service:   svc,
		Timestamp: common.Now(),
	})
}
