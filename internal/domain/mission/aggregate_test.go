package mission

import (
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/stretchr/testify/assert"
)

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

func TestMissionAggregate_TimestampsUpdated(t *testing.T) {
	m := &Mission{ID: common.NewID(), Status: MissionCreated}
	agg := NewMissionAggregate(m)

	before := m.UpdatedAt

	assert.NoError(t, agg.TransitionToPlanning("op"))

	assert.True(t, m.UpdatedAt.After(before.Add(-1)))
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
	assert.False(t, isTerminal(MissionRunning))
	assert.False(t, isTerminal(MissionCreated))
	assert.False(t, isTerminal(MissionPlanning))
}
