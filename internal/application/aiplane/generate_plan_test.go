package aiplane

import (
	"context"
	"errors"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAIPlanner struct {
	mock.Mock
}

func (m *mockAIPlanner) GeneratePlan(ctx context.Context, req attackplan.PlanRequest) (*attackplan.PlanResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*attackplan.PlanResponse), args.Error(1)
}

func (m *mockAIPlanner) CorrelateFindings(ctx context.Context, findings []attackplan.Finding) (*attackplan.CorrelationResult, error) {
	args := m.Called(ctx, findings)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*attackplan.CorrelationResult), args.Error(1)
}

func (m *mockAIPlanner) RankPaths(ctx context.Context, graph attackplan.AttackGraph) ([]attackplan.RankedPath, error) {
	args := m.Called(ctx, graph)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]attackplan.RankedPath), args.Error(1)
}

func (m *mockAIPlanner) AdaptStrategy(ctx context.Context, planID string, feedback attackplan.StrategyFeedback) (*attackplan.Strategy, error) {
	args := m.Called(ctx, planID, feedback)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*attackplan.Strategy), args.Error(1)
}

func (m *mockAIPlanner) EvaluateConfidence(ctx context.Context, plan *attackplan.AttackPlan, evidence []attackplan.Evidence) (float64, error) {
	args := m.Called(ctx, plan, evidence)
	return args.Get(0).(float64), args.Error(1)
}

type mockPlanRepo struct {
	mock.Mock
}

func (m *mockPlanRepo) Save(ctx context.Context, plan *attackplan.AttackPlan) error {
	args := m.Called(ctx, plan)
	return args.Error(0)
}

func (m *mockPlanRepo) FindByID(ctx context.Context, id string) (*attackplan.AttackPlan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*attackplan.AttackPlan), args.Error(1)
}

func (m *mockPlanRepo) FindByMission(ctx context.Context, missionID string) ([]*attackplan.AttackPlan, error) {
	args := m.Called(ctx, missionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*attackplan.AttackPlan), args.Error(1)
}

func (m *mockPlanRepo) Update(ctx context.Context, plan *attackplan.AttackPlan) error {
	args := m.Called(ctx, plan)
	return args.Error(0)
}

func (m *mockPlanRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockMissionRepo struct {
	mock.Mock
}

func (m *mockMissionRepo) Save(ctx context.Context, ms *mission.Mission) error {
	args := m.Called(ctx, ms)
	return args.Error(0)
}

func (m *mockMissionRepo) FindByID(ctx context.Context, id string) (*mission.Mission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mission.Mission), args.Error(1)
}

func (m *mockMissionRepo) FindAll(ctx context.Context, filter mission.MissionFilter) ([]*mission.Mission, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mission.Mission), args.Error(1)
}

func (m *mockMissionRepo) Update(ctx context.Context, ms *mission.Mission) error {
	args := m.Called(ctx, ms)
	return args.Error(0)
}

func (m *mockMissionRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockMissionRepo) SaveTask(ctx context.Context, task *mission.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *mockMissionRepo) FindTaskByID(ctx context.Context, id string) (*mission.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mission.Task), args.Error(1)
}

func (m *mockMissionRepo) FindTasksByMission(ctx context.Context, missionID string) ([]*mission.Task, error) {
	args := m.Called(ctx, missionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mission.Task), args.Error(1)
}

func (m *mockMissionRepo) UpdateTask(ctx context.Context, task *mission.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

type mockLLMClient struct {
	mock.Mock
}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

type mockVectorStore struct {
	mock.Mock
}

func (m *mockVectorStore) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	args := m.Called(ctx, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]SearchResult), args.Error(1)
}

type mockEventStore struct {
	mock.Mock
}

func (m *mockEventStore) Append(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func validMissionFixture() *mission.Mission {
	return &mission.Mission{
		ID:   common.NewID(),
		Name: "test-mission",
		Target: mission.Target{
			Name:    "corp-net",
			IPs:     []string{"10.0.0.1", "10.0.0.2"},
			Domains: []string{"example.com"},
			CIDRs:   []string{"10.0.0.0/24"},
		},
		Objectives: []mission.Objective{
			{ID: common.NewID(), Description: "gain initial access", Priority: 1},
		},
		RoE: mission.RoEConstraints{
			MaxSeverity:      common.SeverityHigh,
			AllowDestructive: false,
			AllowPersistence: false,
			AllowExfiltration: true,
		},
		Status:    mission.MissionActive,
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
		StartedBy: "operator-1",
	}
}

func validLLMJSONResponse() string {
	return `{
		"graph": {
			"nodes": [
				{
					"id": "node_1",
					"target_id": "t1",
					"type": "RECON",
					"service": "http",
					"cve": "",
					"confidence": 0.9,
					"risk_level": "LOW"
				},
				{
					"id": "node_2",
					"target_id": "t1",
					"type": "EXPLOIT",
					"service": "http",
					"cve": "CVE-2024-0001",
					"confidence": 0.8,
					"risk_level": "HIGH"
				}
			],
			"edges": [
				{
					"from": "node_1",
					"to": "node_2",
					"weight": 0.9,
					"confidence": 0.85,
					"condition": "service version <= 2.4.0"
				}
			]
		},
		"rationale": "Recon first, then exploit the vulnerable HTTP service"
	}`
}

func TestGeneratePlanUseCase_Execute_Success(t *testing.T) {
	ctx := context.Background()
	m := validMissionFixture()

	mockPlanner := new(mockAIPlanner)
	mockPlanRepo := new(mockPlanRepo)
	mockMissionRepo := new(mockMissionRepo)
	mockLLM := new(mockLLMClient)
	mockVec := new(mockVectorStore)
	mockEvents := new(mockEventStore)

	mockMissionRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)
	mockVec.On("Search", ctx, mock.Anything, 5).Return([]SearchResult{}, nil)
	mockLLM.On("Generate", ctx, mock.Anything).Return(validLLMJSONResponse(), nil)

	mockPlanner.On("RankPaths", ctx, mock.Anything).Return([]attackplan.RankedPath{
		{Nodes: []string{"node_1", "node_2"}, TotalScore: 0.85, Confidence: 0.85, RiskLevel: common.RiskHigh, Steps: 2},
	}, nil)
	mockPlanner.On("EvaluateConfidence", ctx, mock.AnythingOfType("*attackplan.AttackPlan"), mock.Anything).Return(0.85, nil)

	mockPlanRepo.On("Save", ctx, mock.AnythingOfType("*attackplan.AttackPlan")).Return(nil)
	mockEvents.On("Append", ctx, mock.AnythingOfType("attackplan.PlanGenerated")).Return(nil)

	uc := NewGeneratePlanUseCase(mockPlanner, mockPlanRepo, mockMissionRepo, mockLLM, mockVec, mockEvents)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: m.ID.String()})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Plan)
	assert.NotEmpty(t, resp.Plan.PlanID)
	assert.Equal(t, m.ID.String(), resp.Plan.MissionID)
	assert.Len(t, resp.Plan.Nodes, 2)
	assert.Len(t, resp.Plan.Edges, 1)
	assert.Len(t, resp.Plan.RankedPaths, 1)
	assert.Equal(t, 0.85, resp.Plan.Confidence)
	assert.Equal(t, "DRAFT", resp.Plan.Status)

	mockMissionRepo.AssertExpectations(t)
	mockLLM.AssertExpectations(t)
	mockPlanner.AssertExpectations(t)
	mockPlanRepo.AssertExpectations(t)
	mockEvents.AssertExpectations(t)
}

func TestGeneratePlanUseCase_Execute_EmptyMissionID(t *testing.T) {
	uc := NewGeneratePlanUseCase(nil, nil, nil, nil, nil, nil)

	resp, err := uc.Execute(context.Background(), ExecuteRequest{MissionID: ""})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidID)
}

func TestGeneratePlanUseCase_Execute_MissionNotFound(t *testing.T) {
	ctx := context.Background()
	mockMissionRepo := new(mockMissionRepo)
	mockMissionRepo.On("FindByID", ctx, "bad-id").Return(nil, common.ErrMissionNotFound)

	uc := NewGeneratePlanUseCase(nil, nil, mockMissionRepo, nil, nil, nil)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: "bad-id"})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrMissionNotFound)
}

func TestGeneratePlanUseCase_Execute_InvalidMissionStatus(t *testing.T) {
	ctx := context.Background()
	m := validMissionFixture()
	m.Status = mission.MissionCompleted

	mockMissionRepo := new(mockMissionRepo)
	mockMissionRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	uc := NewGeneratePlanUseCase(nil, nil, mockMissionRepo, nil, nil, nil)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: m.ID.String()})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidState)
}

func TestGeneratePlanUseCase_Execute_NoTargetDefined(t *testing.T) {
	ctx := context.Background()
	m := validMissionFixture()
	m.Target = mission.Target{Name: "empty"}

	mockMissionRepo := new(mockMissionRepo)
	mockMissionRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	uc := NewGeneratePlanUseCase(nil, nil, mockMissionRepo, nil, nil, nil)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: m.ID.String()})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidTarget)
}

func TestGeneratePlanUseCase_Execute_LLMClientError(t *testing.T) {
	ctx := context.Background()
	m := validMissionFixture()

	mockMissionRepo := new(mockMissionRepo)
	mockMissionRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	mockVec := new(mockVectorStore)
	mockVec.On("Search", ctx, mock.Anything, 5).Return([]SearchResult{}, nil)

	mockLLM := new(mockLLMClient)
	mockLLM.On("Generate", ctx, mock.Anything).Return("", errors.New("llm timeout"))

	uc := NewGeneratePlanUseCase(nil, nil, mockMissionRepo, mockLLM, mockVec, nil)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: m.ID.String()})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm generate")
}

func TestGeneratePlanUseCase_Execute_PlanSaveError(t *testing.T) {
	ctx := context.Background()
	m := validMissionFixture()

	mockMissionRepo := new(mockMissionRepo)
	mockMissionRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	mockVec := new(mockVectorStore)
	mockVec.On("Search", ctx, mock.Anything, 5).Return([]SearchResult{}, nil)

	mockLLM := new(mockLLMClient)
	mockLLM.On("Generate", ctx, mock.Anything).Return(validLLMJSONResponse(), nil)

	mockPlanner := new(mockAIPlanner)
	mockPlanner.On("RankPaths", ctx, mock.Anything).Return([]attackplan.RankedPath{}, nil)
	mockPlanner.On("EvaluateConfidence", ctx, mock.AnythingOfType("*attackplan.AttackPlan"), mock.Anything).Return(0.5, nil)

	mockPlanRepo := new(mockPlanRepo)
	mockPlanRepo.On("Save", ctx, mock.AnythingOfType("*attackplan.AttackPlan")).Return(errors.New("db connection refused"))

	uc := NewGeneratePlanUseCase(mockPlanner, mockPlanRepo, mockMissionRepo, mockLLM, mockVec, nil)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: m.ID.String()})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save plan")
}

func TestGeneratePlanUseCase_Execute_EventStoreError(t *testing.T) {
	ctx := context.Background()
	m := validMissionFixture()

	mockMissionRepo := new(mockMissionRepo)
	mockMissionRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	mockVec := new(mockVectorStore)
	mockVec.On("Search", ctx, mock.Anything, 5).Return([]SearchResult{}, nil)

	mockLLM := new(mockLLMClient)
	mockLLM.On("Generate", ctx, mock.Anything).Return(validLLMJSONResponse(), nil)

	mockPlanner := new(mockAIPlanner)
	mockPlanner.On("RankPaths", ctx, mock.Anything).Return([]attackplan.RankedPath{}, nil)
	mockPlanner.On("EvaluateConfidence", ctx, mock.AnythingOfType("*attackplan.AttackPlan"), mock.Anything).Return(0.5, nil)

	mockPlanRepo := new(mockPlanRepo)
	mockPlanRepo.On("Save", ctx, mock.AnythingOfType("*attackplan.AttackPlan")).Return(nil)

	mockEvents := new(mockEventStore)
	mockEvents.On("Append", ctx, mock.AnythingOfType("attackplan.PlanGenerated")).Return(errors.New("event store unavailable"))

	uc := NewGeneratePlanUseCase(mockPlanner, mockPlanRepo, mockMissionRepo, mockLLM, mockVec, mockEvents)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: m.ID.String()})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "append event")
}

func TestGeneratePlanUseCase_Execute_WithVectorContext(t *testing.T) {
	ctx := context.Background()
	m := validMissionFixture()

	mockMissionRepo := new(mockMissionRepo)
	mockMissionRepo.On("FindByID", ctx, m.ID.String()).Return(m, nil)

	mockVec := new(mockVectorStore)
	mockVec.On("Search", ctx, mock.Anything, 5).Return([]SearchResult{
		{ID: "ttp-1", Content: "CVE-2024-0001 RCE in Apache 2.4", Score: 0.95, Metadata: map[string]interface{}{"technique": "T1190"}},
		{ID: "ttp-2", Content: "CVE-2024-0002 privilege escalation", Score: 0.87, Metadata: map[string]interface{}{"technique": "T1068"}},
	}, nil)

	mockLLM := new(mockLLMClient)
	mockLLM.On("Generate", ctx, mock.Anything).Return(validLLMJSONResponse(), nil)

	mockPlanner := new(mockAIPlanner)
	mockPlanner.On("RankPaths", ctx, mock.Anything).Return([]attackplan.RankedPath{
		{Nodes: []string{"node_1", "node_2"}, TotalScore: 0.91, Confidence: 0.91, RiskLevel: common.RiskHigh, Steps: 2},
	}, nil)
	mockPlanner.On("EvaluateConfidence", ctx, mock.AnythingOfType("*attackplan.AttackPlan"), mock.Anything).Return(0.91, nil)

	mockPlanRepo := new(mockPlanRepo)
	mockPlanRepo.On("Save", ctx, mock.AnythingOfType("*attackplan.AttackPlan")).Return(nil)

	mockEvents := new(mockEventStore)
	mockEvents.On("Append", ctx, mock.AnythingOfType("attackplan.PlanGenerated")).Return(nil)

	uc := NewGeneratePlanUseCase(mockPlanner, mockPlanRepo, mockMissionRepo, mockLLM, mockVec, mockEvents)

	resp, err := uc.Execute(ctx, ExecuteRequest{MissionID: m.ID.String()})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0.91, resp.Plan.Confidence)
}

func TestValidateMissionForPlanning(t *testing.T) {
	tests := []struct {
		name    string
		mission *mission.Mission
		wantErr bool
	}{
		{
			name: "valid draft",
			mission: &mission.Mission{
				Status:     mission.MissionDraft,
				Objectives: []mission.Objective{{Description: "test"}},
			},
			wantErr: false,
		},
		{
			name: "valid active",
			mission: &mission.Mission{
				Status:     mission.MissionActive,
				Objectives: []mission.Objective{{Description: "test"}},
			},
			wantErr: false,
		},
		{
			name: "invalid completed",
			mission: &mission.Mission{
				Status:     mission.MissionCompleted,
				Objectives: []mission.Objective{{Description: "test"}},
			},
			wantErr: true,
		},
		{
			name: "invalid no objectives",
			mission: &mission.Mission{
				Status: mission.MissionActive,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMissionForPlanning(tt.mission)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMissionTarget(t *testing.T) {
	tests := []struct {
		name    string
		mission *mission.Mission
		wantErr bool
	}{
		{
			name: "valid with ips",
			mission: &mission.Mission{
				Target: mission.Target{Name: "t", IPs: []string{"10.0.0.1"}},
			},
			wantErr: false,
		},
		{
			name: "valid with domains",
			mission: &mission.Mission{
				Target: mission.Target{Name: "t", Domains: []string{"example.com"}},
			},
			wantErr: false,
		},
		{
			name: "valid with cidrs",
			mission: &mission.Mission{
				Target: mission.Target{Name: "t", CIDRs: []string{"10.0.0.0/24"}},
			},
			wantErr: false,
		},
		{
			name: "empty target data",
			mission: &mission.Mission{
				Target: mission.Target{Name: "empty"},
			},
			wantErr: true,
		},
		{
			name: "missing target name",
			mission: &mission.Mission{
				Target: mission.Target{IPs: []string{"10.0.0.1"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMissionTarget(tt.mission)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseLLMOutput_ValidJSON(t *testing.T) {
	raw := validLLMJSONResponse()
	result, err := parseLLMOutput(raw)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Graph.Nodes, 2)
	assert.Len(t, result.Graph.Edges, 1)
	assert.Equal(t, "Recon first, then exploit the vulnerable HTTP service", result.Rationale)
}

func TestParseLLMOutput_JSONWithWrapping(t *testing.T) {
	raw := "Here is your plan:\n```json\n" + validLLMJSONResponse() + "\n```\nHope this helps!"
	result, err := parseLLMOutput(raw)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Graph.Nodes, 2)
}

func TestParseLLMOutput_NoJSON(t *testing.T) {
	raw := "I cannot generate a plan for this target."
	result, err := parseLLMOutput(raw)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no json found")
}

func TestParseLLMOutput_InvalidJSON(t *testing.T) {
	raw := `{"graph": {"nodes": [{"id": "n1", "malformed": true}]} "extra"`
	result, err := parseLLMOutput(raw)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no json found")
}

func TestBuildTargetProfile(t *testing.T) {
	m := validMissionFixture()
	profile := buildTargetProfile(m)

	assert.Equal(t, m.Target.IPs, profile.IPs)
	assert.Equal(t, m.Target.Domains, profile.Domains)
	assert.Contains(t, profile.Tags, m.Target.Name)
}

func TestMapRoE(t *testing.T) {
	roe := mission.RoEConstraints{
		MaxSeverity:      common.SeverityHigh,
		AllowDestructive: true,
		AllowPersistence: false,
		AllowExfiltration: true,
	}

	result := mapRoE(roe)

	assert.Equal(t, string(common.SeverityHigh), result.MaxSeverity)
	assert.True(t, result.AllowDestructive)
	assert.False(t, result.AllowPersistence)
	assert.True(t, result.AllowExfiltration)
}

func TestBuildPastAttempts(t *testing.T) {
	tasks := []mission.Task{
		{
			ID:        common.NewID(),
			MissionID: common.NewID(),
			Status:    common.TaskSuccess,
			CreatedAt: common.Now(),
		},
		{
			ID:        common.NewID(),
			MissionID: common.NewID(),
			Status:    common.TaskFailed,
			CreatedAt: common.Now(),
			Result: &mission.TaskResult{
				Error: &mission.TaskError{Message: "connection refused"},
			},
		},
		{
			ID:        common.NewID(),
			MissionID: common.NewID(),
			Status:    common.TaskRunning,
			CreatedAt: common.Now(),
		},
	}

	attempts := buildPastAttempts(tasks)

	assert.Len(t, attempts, 2)
	assert.Equal(t, "success", attempts[0].Result)
	assert.Equal(t, "failed", attempts[1].Result)
	assert.Equal(t, "connection refused", attempts[1].Error)
}

func TestParseRiskLevel(t *testing.T) {
	assert.Equal(t, common.RiskCritical, parseRiskLevel("CRITICAL"))
	assert.Equal(t, common.RiskHigh, parseRiskLevel("HIGH"))
	assert.Equal(t, common.RiskMedium, parseRiskLevel("MEDIUM"))
	assert.Equal(t, common.RiskLow, parseRiskLevel("LOW"))
	assert.Equal(t, common.RiskLow, parseRiskLevel("unknown"))
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean json",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json in text",
			input:    "before text {\"key\": \"value\"} after text",
			expected: `{"key": "value"}`,
		},
		{
			name:     "nested json",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "no json",
			input:    "no json here",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
