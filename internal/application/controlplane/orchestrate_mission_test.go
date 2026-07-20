package controlplane

import (
	"context"
	"errors"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockOrchMissionRepo struct {
	mock.Mock
}

func (m *mockOrchMissionRepo) Save(ctx context.Context, ms *mission.Mission) error {
	args := m.Called(ctx, ms)
	return args.Error(0)
}

func (m *mockOrchMissionRepo) FindByID(ctx context.Context, id string) (*mission.Mission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mission.Mission), args.Error(1)
}

func (m *mockOrchMissionRepo) FindAll(ctx context.Context, filter mission.MissionFilter) ([]*mission.Mission, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mission.Mission), args.Error(1)
}

func (m *mockOrchMissionRepo) Update(ctx context.Context, ms *mission.Mission) error {
	args := m.Called(ctx, ms)
	return args.Error(0)
}

func (m *mockOrchMissionRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockOrchMissionRepo) SaveTask(ctx context.Context, task *mission.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *mockOrchMissionRepo) FindTaskByID(ctx context.Context, id string) (*mission.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mission.Task), args.Error(1)
}

func (m *mockOrchMissionRepo) FindTasksByMission(ctx context.Context, missionID string) ([]*mission.Task, error) {
	args := m.Called(ctx, missionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mission.Task), args.Error(1)
}

func (m *mockOrchMissionRepo) UpdateTask(ctx context.Context, task *mission.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

type mockOrchEventStore struct {
	mock.Mock
}

func (m *mockOrchEventStore) Append(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockOrchEventStore) Replay(ctx context.Context, aggregateID string) ([]interface{}, error) {
	args := m.Called(ctx, aggregateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]interface{}), args.Error(1)
}

func createdMissionFixture() *mission.Mission {
	return &mission.Mission{
		ID:        common.NewID(),
		Name:      "test-mission",
		Status:    mission.MissionCreated,
		StartedBy: "operator-1",
		Target: mission.Target{
			Name: "corp-net",
			IPs:  []string{"10.0.0.1"},
		},
		RoE: mission.RoEConstraints{
			MaxSeverity: common.SeverityHigh,
		},
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
	}
}

func TestOrchestrateMissionUseCase_PlanSuccess(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionPlan,
		UpdatedBy: "operator-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Mission)
	assert.Equal(t, m.ID.String(), resp.Mission.ID)
	assert.Equal(t, string(mission.MissionPlanning), resp.Mission.Status)
	assert.NotEmpty(t, resp.Mission.UpdatedAt)

	mockRepo.AssertExpectations(t)
	mockEvt.AssertExpectations(t)
}

func TestOrchestrateMissionUseCase_ReadySuccess(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()
	m.Status = mission.MissionPlanning

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionReady,
		UpdatedBy: "operator-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, string(mission.MissionReady), resp.Mission.Status)
}

func TestOrchestrateMissionUseCase_RunSuccess(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()
	m.Status = mission.MissionReady

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionRun,
		UpdatedBy: "operator-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, string(mission.MissionRunning), resp.Mission.Status)
}

func TestOrchestrateMissionUseCase_CompleteSuccess(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()
	m.Status = mission.MissionRunning

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionComplete,
		UpdatedBy: "operator-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, string(mission.MissionCompleted), resp.Mission.Status)
}

func TestOrchestrateMissionUseCase_FailSuccess(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()
	m.Status = mission.MissionRunning

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionFail,
		UpdatedBy: "operator-1",
		Reason:    "connection lost",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, string(mission.MissionFailed), resp.Mission.Status)
}

func TestOrchestrateMissionUseCase_EmptyMissionID(t *testing.T) {
	uc := NewOrchestrateMissionUseCase(nil, nil)

	resp, err := uc.Execute(context.Background(), OrchestrateRequest{
		Action: ActionPlan,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidID)
}

func TestOrchestrateMissionUseCase_EmptyAction(t *testing.T) {
	uc := NewOrchestrateMissionUseCase(nil, nil)

	resp, err := uc.Execute(context.Background(), OrchestrateRequest{
		MissionID: "m1",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidState)
}

func TestOrchestrateMissionUseCase_MissionNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, "bad-id").Return(nil, common.ErrMissionNotFound)

	uc := NewOrchestrateMissionUseCase(mockRepo, nil)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: "bad-id",
		Action:    ActionPlan,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrMissionNotFound)
}

func TestOrchestrateMissionUseCase_InvalidTransition(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()
	m.Status = mission.MissionRunning

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, nil)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionPlan,
		UpdatedBy: "op",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTransition)
}

func TestOrchestrateMissionUseCase_UnknownAction(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, nil)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    "INVALID",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidState)
}

func TestOrchestrateMissionUseCase_UpdateError(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(errors.New("db error"))

	uc := NewOrchestrateMissionUseCase(mockRepo, nil)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionPlan,
		UpdatedBy: "op",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persist mission")
}

func TestOrchestrateMissionUseCase_EventStoreError(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(errors.New("event store down"))

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionPlan,
		UpdatedBy: "op",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "append event")
}

func TestMapToMissionLifecycleDTO(t *testing.T) {
	m := createdMissionFixture()
	m.Status = mission.MissionRunning

	dto := mapToMissionLifecycleDTO(m)

	assert.Equal(t, m.ID.String(), dto.ID)
	assert.Equal(t, m.Name, dto.Name)
	assert.Equal(t, string(mission.MissionRunning), dto.Status)
	assert.Equal(t, m.StartedBy, dto.StartedBy)
	assert.NotEmpty(t, dto.CreatedAt)
	assert.NotEmpty(t, dto.UpdatedAt)
}

func TestOrchestrateMissionUseCase_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil).Times(5)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil).Times(5)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(nil).Times(5)

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{MissionID: m.ID.String(), Action: ActionPlan, UpdatedBy: "op"})
	assert.NoError(t, err)
	assert.Equal(t, string(mission.MissionPlanning), resp.Mission.Status)

	m.Status = mission.MissionPlanning
	resp, err = uc.Execute(ctx, OrchestrateRequest{MissionID: m.ID.String(), Action: ActionReady, UpdatedBy: "op"})
	assert.NoError(t, err)
	assert.Equal(t, string(mission.MissionReady), resp.Mission.Status)

	m.Status = mission.MissionReady
	resp, err = uc.Execute(ctx, OrchestrateRequest{MissionID: m.ID.String(), Action: ActionRun, UpdatedBy: "op"})
	assert.NoError(t, err)
	assert.Equal(t, string(mission.MissionRunning), resp.Mission.Status)

	m.Status = mission.MissionRunning
	resp, err = uc.Execute(ctx, OrchestrateRequest{MissionID: m.ID.String(), Action: ActionComplete, UpdatedBy: "op"})
	assert.NoError(t, err)
	assert.Equal(t, string(mission.MissionCompleted), resp.Mission.Status)
}

func TestOrchestrateMissionUseCase_FailAfterRunning(t *testing.T) {
	ctx := context.Background()
	m := createdMissionFixture()
	m.Status = mission.MissionRunning

	mockRepo := new(mockOrchMissionRepo)
	mockRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*mission.Mission")).Return(nil)

	mockEvt := new(mockOrchEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("mission.MissionStateChanged")).Return(nil)

	uc := NewOrchestrateMissionUseCase(mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, OrchestrateRequest{
		MissionID: m.ID.String(),
		Action:    ActionFail,
		Reason:    "exploit failed",
		UpdatedBy: "system",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, string(mission.MissionFailed), resp.Mission.Status)
}
