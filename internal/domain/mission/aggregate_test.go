package mission

import (
	"testing"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/stretchr/testify/assert"
)

func TestMissionAggregate_Start_FromDraft(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionDraft, StartedBy: "op"}
	agg := NewMissionAggregate(m)

	err := agg.Start()

	assert.NoError(t, err)
	assert.Equal(t, MissionActive, m.Status)
	assert.Len(t, agg.Events, 1)
	assert.Equal(t, "MissionStarted", agg.Events[0].EventType())

	evt := agg.Events[0].(MissionStarted)
	assert.Equal(t, m.ID, evt.MissionID)
	assert.Equal(t, "op", evt.StartedBy)
}

func TestMissionAggregate_Start_FromCreated(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated, StartedBy: "op"}
	agg := NewMissionAggregate(m)

	err := agg.Start()

	assert.NoError(t, err)
	assert.Equal(t, MissionActive, m.Status)
}

func TestMissionAggregate_Start_InvalidStatus(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionRunning, StartedBy: "op"}
	agg := NewMissionAggregate(m)

	err := agg.Start()

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
}

func TestMissionAggregate_Pause(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionActive}
	agg := NewMissionAggregate(m)

	err := agg.Pause("time window expired", "operator-1")

	assert.NoError(t, err)
	assert.Equal(t, MissionPaused, m.Status)
	assert.Len(t, agg.Events, 1)

	evt := agg.Events[0].(MissionStateChanged)
	assert.Equal(t, MissionActive, evt.FromStatus)
	assert.Equal(t, MissionPaused, evt.ToStatus)
	assert.Equal(t, "time window expired", evt.Reason)
	assert.Equal(t, "operator-1", evt.UpdatedBy)
}

func TestMissionAggregate_Pause_InvalidState(t *testing.T) {
	tests := []MissionStatus{MissionDraft, MissionPaused, MissionCompleted, MissionFailed}
	for _, status := range tests {
		t.Run(string(status), func(t *testing.T) {
			m := &Mission{ID: common.NewID(), Status: status}
			agg := NewMissionAggregate(m)
			err := agg.Pause("reason", "op")
			assert.Error(t, err)
			assert.ErrorIs(t, err, common.ErrInvalidTransition)
		})
	}
}

func TestMissionAggregate_Resume(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionPaused}
	agg := NewMissionAggregate(m)

	err := agg.Resume("operator-1")

	assert.NoError(t, err)
	assert.Equal(t, MissionActive, m.Status)
	assert.Len(t, agg.Events, 1)

	evt := agg.Events[0].(MissionStateChanged)
	assert.Equal(t, MissionPaused, evt.FromStatus)
	assert.Equal(t, MissionActive, evt.ToStatus)
}

func TestMissionAggregate_Resume_InvalidState(t *testing.T) {
	tests := []MissionStatus{MissionActive, MissionDraft, MissionRunning, MissionCompleted}
	for _, status := range tests {
		t.Run(string(status), func(t *testing.T) {
			m := &Mission{ID: common.NewID(), Status: status}
			agg := NewMissionAggregate(m)
			err := agg.Resume("op")
			assert.Error(t, err)
			assert.ErrorIs(t, err, common.ErrInvalidTransition)
		})
	}
}

func TestMissionAggregate_Abort(t *testing.T) {
	tests := []MissionStatus{MissionActive, MissionPaused, MissionCreated, MissionPlanning, MissionReady, MissionRunning}
	for _, status := range tests {
		t.Run(string(status), func(t *testing.T) {
			m := &Mission{ID: common.NewID(), Status: status}
			agg := NewMissionAggregate(m)

			err := agg.Abort("operator decision", "operator-1")

			assert.NoError(t, err)
			assert.Equal(t, MissionAborted, m.Status)
			assert.Len(t, agg.Events, 1)

			evt := agg.Events[0].(MissionStateChanged)
			assert.Equal(t, status, evt.FromStatus)
			assert.Equal(t, MissionAborted, evt.ToStatus)
			assert.Equal(t, "operator decision", evt.Reason)
		})
	}
}

func TestMissionAggregate_Abort_TerminalStates(t *testing.T) {
	tests := []MissionStatus{MissionCompleted, MissionFailed, MissionAborted}
	for _, status := range tests {
		t.Run(string(status), func(t *testing.T) {
			m := &Mission{ID: common.NewID(), Status: status}
			agg := NewMissionAggregate(m)
			err := agg.Abort("reason", "op")
			assert.Error(t, err)
			assert.ErrorIs(t, err, common.ErrInvalidTransition)
		})
	}
}

func TestMissionAggregate_PauseResumeCycle(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionActive}
	agg := NewMissionAggregate(m)

	assert.NoError(t, agg.Pause("pause", "op"))
	assert.Equal(t, MissionPaused, m.Status)

	assert.NoError(t, agg.Resume("op"))
	assert.Equal(t, MissionActive, m.Status)
	assert.Len(t, agg.Events, 2)
}

func TestMissionAggregate_TransitionToPlanning(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated}
	agg := NewMissionAggregate(m)

	err := agg.TransitionToPlanning("operator-1")

	assert.NoError(t, err)
	assert.Equal(t, MissionPlanning, m.Status)
	assert.Len(t, agg.Events, 1)
	assert.Equal(t, "MissionStateChanged", agg.Events[0].EventType())

	evt := agg.Events[0].(MissionStateChanged)
	assert.Equal(t, MissionCreated, evt.FromStatus)
	assert.Equal(t, MissionPlanning, evt.ToStatus)
	assert.Equal(t, "operator-1", evt.UpdatedBy)
}

func TestMissionAggregate_TransitionToPlanning_InvalidState(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionRunning}
	agg := NewMissionAggregate(m)

	err := agg.TransitionToPlanning("operator-1")

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
	assert.Equal(t, MissionRunning, m.Status)
	assert.Len(t, agg.Events, 0)
}

func TestMissionAggregate_TransitionToReady(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionPlanning}
	agg := NewMissionAggregate(m)

	err := agg.TransitionToReady("operator-1")

	assert.NoError(t, err)
	assert.Equal(t, MissionReady, m.Status)
	assert.Len(t, agg.Events, 1)

	evt := agg.Events[0].(MissionStateChanged)
	assert.Equal(t, MissionPlanning, evt.FromStatus)
	assert.Equal(t, MissionReady, evt.ToStatus)
}

func TestMissionAggregate_TransitionToReady_InvalidState(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated}
	agg := NewMissionAggregate(m)

	err := agg.TransitionToReady("operator-1")

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
}

func TestMissionAggregate_TransitionToRunning(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionReady}
	agg := NewMissionAggregate(m)

	err := agg.TransitionToRunning("operator-1")

	assert.NoError(t, err)
	assert.Equal(t, MissionRunning, m.Status)
	assert.Len(t, agg.Events, 1)

	evt := agg.Events[0].(MissionStateChanged)
	assert.Equal(t, MissionReady, evt.FromStatus)
	assert.Equal(t, MissionRunning, evt.ToStatus)
}

func TestMissionAggregate_TransitionToRunning_InvalidState(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionPlanning}
	agg := NewMissionAggregate(m)

	err := agg.TransitionToRunning("operator-1")

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
}

func TestMissionAggregate_Complete(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionRunning}
	agg := NewMissionAggregate(m)

	err := agg.Complete("operator-1")

	assert.NoError(t, err)
	assert.Equal(t, MissionCompleted, m.Status)
	assert.Len(t, agg.Events, 1)

	evt := agg.Events[0].(MissionStateChanged)
	assert.Equal(t, MissionRunning, evt.FromStatus)
	assert.Equal(t, MissionCompleted, evt.ToStatus)
}

func TestMissionAggregate_Complete_InvalidState(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionPlanning}
	agg := NewMissionAggregate(m)

	err := agg.Complete("operator-1")

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
}

func TestMissionAggregate_Fail(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionRunning}
	agg := NewMissionAggregate(m)

	err := agg.Fail("exploit failed", "system")

	assert.NoError(t, err)
	assert.Equal(t, MissionFailed, m.Status)
	assert.Len(t, agg.Events, 1)

	evt := agg.Events[0].(MissionStateChanged)
	assert.Equal(t, MissionRunning, evt.FromStatus)
	assert.Equal(t, MissionFailed, evt.ToStatus)
	assert.Equal(t, "exploit failed", evt.Reason)
	assert.Equal(t, "system", evt.UpdatedBy)
}

func TestMissionAggregate_Fail_InvalidState(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated}
	agg := NewMissionAggregate(m)

	err := agg.Fail("reason", "op")

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
}

func TestMissionAggregate_FullLifecycle(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated}
	agg := NewMissionAggregate(m)

	assert.NoError(t, agg.TransitionToPlanning("op"))
	assert.Equal(t, MissionPlanning, m.Status)
	agg.ClearEvents()

	assert.NoError(t, agg.TransitionToReady("op"))
	assert.Equal(t, MissionReady, m.Status)
	agg.ClearEvents()

	assert.NoError(t, agg.TransitionToRunning("op"))
	assert.Equal(t, MissionRunning, m.Status)
	agg.ClearEvents()

	assert.NoError(t, agg.Complete("op"))
	assert.Equal(t, MissionCompleted, m.Status)
}

func TestMissionAggregate_FullLifecycleWithPause(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated}
	agg := NewMissionAggregate(m)

	assert.NoError(t, agg.Start())
	assert.Equal(t, MissionActive, m.Status)

	assert.NoError(t, agg.Pause("roi-check", "op"))
	assert.Equal(t, MissionPaused, m.Status)

	assert.NoError(t, agg.Resume("op"))
	assert.Equal(t, MissionActive, m.Status)

	assert.NoError(t, agg.Abort("mission cancelled", "admin"))
	assert.Equal(t, MissionAborted, m.Status)

	assert.True(t, isTerminal(m.Status))
}

func TestMissionAggregate_TimestampsUpdated(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated}
	agg := NewMissionAggregate(m)

	before := m.UpdatedAt
	assert.NoError(t, agg.TransitionToPlanning("op"))
	assert.True(t, m.UpdatedAt.After(before.Add(-1)))
}

func TestTransitionPhase_TriggeredBy(t *testing.T) {
	m := &Mission{ID: common.NewID()}
	agg := NewMissionAggregate(m)

	err := agg.TransitionPhase(common.PhaseRecon, common.PhasePlanning, "ai-engine", "auto transition")

	assert.NoError(t, err)
	assert.Len(t, agg.Events, 1)

	evt := agg.Events[0].(PhaseTransitioned)
	assert.Equal(t, common.PhaseRecon, evt.FromPhase)
	assert.Equal(t, common.PhasePlanning, evt.ToPhase)
	assert.Equal(t, "ai-engine", evt.TriggeredBy)
	assert.Equal(t, "auto transition", evt.Reason)
}

func TestTransitionPhase_Invalid(t *testing.T) {
	m := &Mission{ID: common.NewID()}
	agg := NewMissionAggregate(m)

	err := agg.TransitionPhase(common.PhaseRecon, common.PhaseExploit, "op", "skip")

	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
}

func TestValidateRoE_MaxTargets(t *testing.T) {
	roe := RoEConstraints{MaxTargets: 2}
	target := Target{IPs: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}}

	err := ValidateRoE(roe, target)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrRoEViolation)
}

func TestValidateRoE_DomainTargets(t *testing.T) {
	roe := RoEConstraints{MaxTargets: 2}
	target := Target{IPs: []string{"10.0.0.1"}, Domains: []string{"a.com", "b.com"}}

	err := ValidateRoE(roe, target)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrRoEViolation)
}

func TestValidateRoE_TimeWindow(t *testing.T) {
	roe := RoEConstraints{
		MaxTargets:      100,
		TimeWindowStart: common.Now().Add(-1 * time.Hour),
		TimeWindowEnd:   common.Now().Add(-30 * time.Minute),
	}
	target := Target{IPs: []string{"10.0.0.1"}}

	err := ValidateRoE(roe, target)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrRoEViolation)
}

func TestValidateRoE_ExcludedNets(t *testing.T) {
	roe := RoEConstraints{
		ExcludedNets: []string{"10.0.0.1"},
	}
	target := Target{IPs: []string{"10.0.0.1"}}

	err := ValidateRoE(roe, target)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrRoEViolation)
}

func TestValidateRoE_Valid(t *testing.T) {
	roe := RoEConstraints{MaxTargets: 10}
	target := Target{IPs: []string{"10.0.0.1", "10.0.0.2"}}

	err := ValidateRoE(roe, target)
	assert.NoError(t, err)
}

func TestIsValidLifecycleTransition(t *testing.T) {
	assert.True(t, isValidLifecycleTransition(MissionCreated, MissionPlanning))
	assert.True(t, isValidLifecycleTransition(MissionPlanning, MissionReady))
	assert.True(t, isValidLifecycleTransition(MissionReady, MissionRunning))
	assert.True(t, isValidLifecycleTransition(MissionRunning, MissionCompleted))
	assert.True(t, isValidLifecycleTransition(MissionRunning, MissionFailed))

	assert.False(t, isValidLifecycleTransition(MissionCreated, MissionReady))
	assert.False(t, isValidLifecycleTransition(MissionCreated, MissionRunning))
	assert.False(t, isValidLifecycleTransition(MissionPlanning, MissionRunning))
	assert.False(t, isValidLifecycleTransition(MissionReady, MissionCompleted))
	assert.False(t, isValidLifecycleTransition(MissionCompleted, MissionPlanning))
	assert.False(t, isValidLifecycleTransition(MissionDraft, MissionPlanning))
}

func TestIsTerminal(t *testing.T) {
	assert.True(t, isTerminal(MissionCompleted))
	assert.True(t, isTerminal(MissionFailed))
	assert.True(t, isTerminal(MissionAborted))
	assert.False(t, isTerminal(MissionRunning))
	assert.False(t, isTerminal(MissionCreated))
	assert.False(t, isTerminal(MissionPlanning))
	assert.False(t, isTerminal(MissionActive))
	assert.False(t, isTerminal(MissionPaused))
}
