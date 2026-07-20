package mission

import (
	"github.com/ophidian/ophidian/internal/domain/common"
)

type MissionAggregate struct {
	Mission *Mission
	Events  []DomainEvent
	Version int
}

func NewMissionAggregate(mission *Mission) *MissionAggregate {
	return &MissionAggregate{
		Mission: mission,
		Events:  []DomainEvent{},
		Version: 0,
	}
}

func (a *MissionAggregate) AddEvent(event DomainEvent) {
	a.Events = append(a.Events, event)
	a.Version++
}

func (a *MissionAggregate) ClearEvents() {
	a.Events = nil
}

func (a *MissionAggregate) Start() error {
	if a.Mission.Status != MissionDraft {
		return common.ErrInvalidTransition
	}
	a.Mission.Status = MissionActive
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(MissionStarted{
		MissionID: a.Mission.ID,
		Target:    a.Mission.Target,
		Objectives: a.Mission.Objectives,
		RoE:       a.Mission.RoE,
		StartedAt: common.Now(),
		StartedBy: a.Mission.StartedBy,
	})
	return nil
}

func (a *MissionAggregate) TransitionPhase(from, to common.Phase, reason string) error {
	if !isValidTransition(from, to) {
		return common.ErrInvalidTransition
	}
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(PhaseTransitioned{
		MissionID:   a.Mission.ID,
		FromPhase:   from,
		ToPhase:     to,
		Reason:      reason,
		TriggeredBy: "system",
		Timestamp:   common.Now(),
	})
	return nil
}
