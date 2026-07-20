package mission

import (
	"fmt"

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

func (a *MissionAggregate) TransitionToPlanning(updatedBy string) error {
	return a.transitionLifecycle(MissionCreated, MissionPlanning, "transition to planning", updatedBy)
}

func (a *MissionAggregate) TransitionToReady(updatedBy string) error {
	return a.transitionLifecycle(MissionPlanning, MissionReady, "plan approved, ready to execute", updatedBy)
}

func (a *MissionAggregate) TransitionToRunning(updatedBy string) error {
	return a.transitionLifecycle(MissionReady, MissionRunning, "execution started", updatedBy)
}

func (a *MissionAggregate) Complete(updatedBy string) error {
	return a.transitionLifecycle(MissionRunning, MissionCompleted, "all objectives met", updatedBy)
}

func (a *MissionAggregate) Fail(reason string, updatedBy string) error {
	if !isValidLifecycleTransition(a.Mission.Status, MissionFailed) {
		return fmt.Errorf("%w: cannot fail mission in status %s", common.ErrInvalidTransition, a.Mission.Status)
	}
	prev := a.Mission.Status
	a.Mission.Status = MissionFailed
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(MissionStateChanged{
		MissionID:  a.Mission.ID,
		FromStatus: prev,
		ToStatus:   MissionFailed,
		Reason:     reason,
		UpdatedBy:  updatedBy,
		Timestamp:  common.Now(),
	})
	return nil
}

func (a *MissionAggregate) transitionLifecycle(expectedFrom, to MissionStatus, reason, updatedBy string) error {
	if a.Mission.Status != expectedFrom {
		return fmt.Errorf("%w: expected status %s, got %s", common.ErrInvalidTransition, expectedFrom, a.Mission.Status)
	}
	a.Mission.Status = to
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(MissionStateChanged{
		MissionID:  a.Mission.ID,
		FromStatus: expectedFrom,
		ToStatus:   to,
		Reason:     reason,
		UpdatedBy:  updatedBy,
		Timestamp:  common.Now(),
	})
	return nil
}
