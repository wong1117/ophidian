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
	if a.Mission.Status != MissionDraft && a.Mission.Status != MissionCreated {
		return fmt.Errorf("%w: cannot start from status %s", common.ErrInvalidTransition, a.Mission.Status)
	}
	a.Mission.Status = MissionActive
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(MissionStarted{
		MissionID: a.Mission.ID,
		StartedAt: common.Now(),
		StartedBy: a.Mission.StartedBy,
	})
	return nil
}

func (a *MissionAggregate) Pause(reason, updatedBy string) error {
	if a.Mission.Status != MissionActive {
		return fmt.Errorf("%w: can only pause active missions, current status is %s", common.ErrInvalidTransition, a.Mission.Status)
	}
	prev := a.Mission.Status
	a.Mission.Status = MissionPaused
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(MissionStateChanged{
		MissionID:  a.Mission.ID,
		FromStatus: prev,
		ToStatus:   MissionPaused,
		Reason:     reason,
		UpdatedBy:  updatedBy,
		Timestamp:  common.Now(),
	})
	return nil
}

func (a *MissionAggregate) Resume(updatedBy string) error {
	if a.Mission.Status != MissionPaused {
		return fmt.Errorf("%w: can only resume paused missions, current status is %s", common.ErrInvalidTransition, a.Mission.Status)
	}
	prev := a.Mission.Status
	a.Mission.Status = MissionActive
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(MissionStateChanged{
		MissionID:  a.Mission.ID,
		FromStatus: prev,
		ToStatus:   MissionActive,
		Reason:     "mission resumed",
		UpdatedBy:  updatedBy,
		Timestamp:  common.Now(),
	})
	return nil
}

func (a *MissionAggregate) Abort(reason, updatedBy string) error {
	if isTerminal(a.Mission.Status) {
		return fmt.Errorf("%w: cannot abort mission in terminal status %s", common.ErrInvalidTransition, a.Mission.Status)
	}
	prev := a.Mission.Status
	a.Mission.Status = MissionAborted
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(MissionStateChanged{
		MissionID:  a.Mission.ID,
		FromStatus: prev,
		ToStatus:   MissionAborted,
		Reason:     reason,
		UpdatedBy:  updatedBy,
		Timestamp:  common.Now(),
	})
	return nil
}

func (a *MissionAggregate) TransitionPhase(from, to common.Phase, triggeredBy, reason string) error {
	if !isValidTransition(from, to) {
		return common.ErrInvalidTransition
	}
	a.Mission.UpdatedAt = common.Now()
	a.AddEvent(PhaseTransitioned{
		MissionID:   a.Mission.ID,
		FromPhase:   from,
		ToPhase:     to,
		Reason:      reason,
		TriggeredBy: triggeredBy,
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
