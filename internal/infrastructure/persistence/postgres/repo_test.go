package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/domain/session"
	"github.com/ophidian/ophidian/internal/domain/target"
	"github.com/stretchr/testify/assert"
)

type mockRows struct {
	data   []mockRowData
	err    error
	idx    int
	closed bool
}

type mockRowData struct{}

func newMockRows(data []mockRowData) *mockRows {
	return &mockRows{data: data, idx: -1}
}

func (r *mockRows) Close()                                        { r.closed = true }
func (r *mockRows) Err() error                                    { return r.err }
func (r *mockRows) CommandTag() pgconn.CommandTag                 { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Next() bool                                    { r.idx++; return r.idx < len(r.data) }
func (r *mockRows) Scan(dest ...any) error {
	for _, dp := range dest {
		switch v := dp.(type) {
		case *[]byte:
			*v = []byte("null")
		case *common.ID:
			*v = common.NewID()
		case *string:
			*v = ""
		case *int:
			*v = 0
		case *float64:
			*v = 0
		}
	}
	return nil
}
func (r *mockRows) Values() ([]any, error)                         { return nil, nil }
func (r *mockRows) RawValues() [][]byte                            { return nil }
func (r *mockRows) Conn() *pgx.Conn                                { return nil }

func missionScanFn(m *mission.Mission) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*common.ID) = m.ID
		*dest[1].(*string) = m.Name
		*dest[2].(*mission.MissionStatus) = m.Status
		*dest[3].(*[]byte) = marshalJSON(m.Target)
		*dest[4].(*[]byte) = marshalJSON(m.RoE)
		*dest[5].(*[]byte) = marshalJSON(m.Phases)
		*dest[6].(*common.UTCTime) = m.CreatedAt
		*dest[7].(*common.UTCTime) = m.UpdatedAt
		*dest[8].(*string) = m.StartedBy
		return nil
	}
}

func TestMissionRepository_Save_Success(t *testing.T) {
	ctx := context.Background()
	execCalled := false

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			assert.Contains(t, sql, "INSERT INTO missions")
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	m := &mission.Mission{
		ID:        common.NewID(),
		Name:      "test",
		Status:    mission.MissionCreated,
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
		Target:    mission.Target{Name: "t"},
	}

	err := repo.Save(ctx, m)
	assert.NoError(t, err)
	assert.True(t, execCalled)
}

func TestMissionRepository_Save_Error(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	err := repo.Save(ctx, &mission.Mission{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save mission")
}

func TestMissionRepository_FindByID_Success(t *testing.T) {
	ctx := context.Background()
	m := &mission.Mission{
		ID:        common.NewID(),
		Name:      "test-mission",
		Status:    mission.MissionActive,
		StartedBy: "op",
		Target:    mission.Target{Name: "corp", IPs: []string{"10.0.0.1"}},
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
	}

	deps := RepoDeps{
		QueryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{scanFn: missionScanFn(m)}
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	result, err := repo.FindByID(ctx, m.ID.String())

	assert.NoError(t, err)
	assert.Equal(t, m.ID, result.ID)
	assert.Equal(t, m.Name, result.Name)
	assert.Equal(t, "corp", result.Target.Name)
}

func TestMissionRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		QueryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	result, err := repo.FindByID(ctx, "bad-id")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrMissionNotFound)
}

func TestMissionRepository_FindAll_WithFilter(t *testing.T) {
	ctx := context.Background()
	m := &mission.Mission{ID: common.NewID(), Name: "test", Status: mission.MissionActive}

	deps := RepoDeps{
		Query: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return newMockRows([]mockRowData{
				{}, // version not needed here, just count
			}), nil
		},
	}

	_ = m
	repo := NewMissionRepositoryWithDeps(deps)
	status := mission.MissionActive
	results, err := repo.FindAll(ctx, mission.MissionFilter{Status: &status, Limit: 10})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMissionRepository_Update_Success(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			assert.Contains(t, sql, "UPDATE missions")
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	m := &mission.Mission{ID: common.NewID(), Name: "updated", UpdatedAt: common.Now()}
	err := repo.Update(ctx, m)
	assert.NoError(t, err)
}

func TestMissionRepository_Update_NotFound(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	err := repo.Update(ctx, &mission.Mission{ID: common.NewID()})
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrMissionNotFound)
}

func TestMissionRepository_Delete_Success(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	err := repo.Delete(ctx, "some-id")
	assert.NoError(t, err)
}

func TestMissionRepository_Delete_NotFound(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	err := repo.Delete(ctx, "bad-id")
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrMissionNotFound)
}

func TestMissionRepository_SaveTask_Success(t *testing.T) {
	ctx := context.Background()
	execCalled := false

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	task := &mission.Task{
		ID:        common.NewID(),
		MissionID: common.NewID(),
		Type:      common.TaskExploit,
		Status:    common.TaskPending,
		CreatedAt: common.Now(),
	}

	err := repo.SaveTask(ctx, task)
	assert.NoError(t, err)
	assert.True(t, execCalled)
}

func TestMissionRepository_FindTaskByID_NotFound(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		QueryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	repo := NewMissionRepositoryWithDeps(deps)
	result, err := repo.FindTaskByID(ctx, "bad-id")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrTaskNotFound)
}

func TestTargetRepository_Save_Upsert(t *testing.T) {
	ctx := context.Background()
	execCalled := false

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			assert.Contains(t, sql, "ON CONFLICT")
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewTargetRepositoryWithDeps(deps)
	trgt := &target.Target{
		ID:        common.NewID(),
		OS:        "linux",
		IPs:       []target.IP{{Address: "10.0.0.1", Type: target.IPv4}},
		Domains:   []target.Domain{{Name: "example.com", TLD: "com"}},
		Services:  []target.Service{{Port: 443, Name: "https", Protocol: "tcp"}},
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
	}

	err := repo.Save(ctx, trgt)
	assert.NoError(t, err)
	assert.True(t, execCalled)
}

func TestTargetRepository_FindByID_Success(t *testing.T) {
	ctx := context.Background()
	trgt := &target.Target{
		ID:        common.NewID(),
		OS:        "linux",
		IPs:       []target.IP{{Address: "10.0.0.1"}},
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
	}

	deps := RepoDeps{
		QueryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{scanFn: func(dest ...any) error {
				*dest[0].(*common.ID) = trgt.ID
				*dest[1].(*[]byte) = marshalJSON(trgt.IPs)
				*dest[2].(*[]byte) = marshalJSON(trgt.Domains)
				*dest[3].(*[]byte) = marshalJSON(trgt.Hostnames)
				*dest[4].(*string) = trgt.OS
				*dest[5].(*[]byte) = marshalJSON(trgt.Services)
				*dest[6].(*[]byte) = marshalJSON(trgt.Tags)
				*dest[7].(*common.UTCTime) = trgt.CreatedAt
				*dest[8].(*common.UTCTime) = trgt.UpdatedAt
				return nil
			}}
		},
	}

	repo := NewTargetRepositoryWithDeps(deps)
	result, err := repo.FindByID(ctx, trgt.ID.String())

	assert.NoError(t, err)
	assert.Equal(t, trgt.ID, result.ID)
	assert.Equal(t, "linux", result.OS)
	assert.Len(t, result.IPs, 1)
}

func TestTargetRepository_FindByIP_NotFound(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		QueryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	repo := NewTargetRepositoryWithDeps(deps)
	result, err := repo.FindByIP(ctx, "10.0.0.1")
	assert.Nil(t, result)
	assert.Error(t, err)
}

func TestAttackPlanRepository_Save_Success(t *testing.T) {
	ctx := context.Background()
	execCalled := false

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewAttackPlanRepositoryWithDeps(deps)
	p := &attackplan.AttackPlan{
		ID:          common.NewID(),
		MissionID:   common.NewID(),
		Graph:       attackplan.AttackGraph{Nodes: []attackplan.Node{{ID: "n1"}}},
		RankedPaths: []attackplan.RankedPath{},
		Confidence:  0.85,
		Status:      attackplan.PlanDraft,
		CreatedAt:   common.Now(),
		UpdatedAt:   common.Now(),
	}

	err := repo.Save(ctx, p)
	assert.NoError(t, err)
	assert.True(t, execCalled)
}

func TestAttackPlanRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		QueryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	repo := NewAttackPlanRepositoryWithDeps(deps)
	result, err := repo.FindByID(ctx, "bad-id")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrPlanNotFound)
}

func TestAttackPlanRepository_FindByMission(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Query: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return newMockRows([]mockRowData{
				{},
			}), nil
		},
	}

	repo := NewAttackPlanRepositoryWithDeps(deps)
	results, err := repo.FindByMission(ctx, "mission-1")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSessionRepository_Save_Success(t *testing.T) {
	ctx := context.Background()
	execCalled := false

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewSessionRepositoryWithDeps(deps)
	sess := &session.Session{
		ID:             common.NewID(),
		MissionID:      common.NewID(),
		TargetID:       common.NewID(),
		Type:           session.SessionReverseShell,
		Protocol:       session.ProtocolTCP,
		Host:           "10.0.0.1",
		Port:           4444,
		PrivilegeLevel: session.PrivilegeUser,
		Status:         session.SessionActive,
		EstablishedAt:  common.Now(),
		LastActiveAt:   common.Now(),
	}

	err := repo.Save(ctx, sess)
	assert.NoError(t, err)
	assert.True(t, execCalled)
}

func TestSessionRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		QueryRow: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return testRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	repo := NewSessionRepositoryWithDeps(deps)
	result, err := repo.FindByID(ctx, "bad-id")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrSessionNotFound)
}

func TestSessionRepository_FindActive(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Query: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return newMockRows([]mockRowData{
				{},
			}), nil
		},
	}

	repo := NewSessionRepositoryWithDeps(deps)
	results, err := repo.FindActive(ctx)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestFindingRepository_Save_Success(t *testing.T) {
	ctx := context.Background()
	execCalled := false

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewFindingRepositoryWithDeps(deps)
	f := &finding.Finding{
		ID:          common.NewID(),
		MissionID:   common.NewID(),
		TargetID:    common.NewID(),
		Title:       "SQL Injection",
		Description: "Found SQLi on login page",
		Severity:    common.SeverityHigh,
		CVSS:        7.5,
		CVE:         "CVE-2024-0001",
		Confidence:  finding.ConfidenceHigh,
		Status:      finding.FindingStatusConfirmed,
		CreatedAt:   common.Now(),
		UpdatedAt:   common.Now(),
	}

	err := repo.Save(ctx, f)
	assert.NoError(t, err)
	assert.True(t, execCalled)
}

func TestFindingRepository_FindByMission(t *testing.T) {
	ctx := context.Background()

	deps := RepoDeps{
		Query: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return newMockRows([]mockRowData{
				{},
			}), nil
		},
	}

	repo := NewFindingRepositoryWithDeps(deps)
	results, err := repo.FindByMission(ctx, "mission-1")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestFindingRepository_SaveEvidence(t *testing.T) {
	ctx := context.Background()
	execCalled := false

	deps := RepoDeps{
		Exec: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			return pgconn.CommandTag{}, nil
		},
	}

	repo := NewFindingRepositoryWithDeps(deps)
	ev := &finding.Evidence{
		ID:        common.NewID(),
		FindingID: common.NewID(),
		Type:      finding.EvidenceLog,
		Data:      []byte("output log"),
		Source:    "exploit-1",
		Timestamp: common.Now(),
		Hash:      "abc123",
	}

	err := repo.SaveEvidence(ctx, ev)
	assert.NoError(t, err)
	assert.True(t, execCalled)
}

func TestMarshalUnmarshalHelpers(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	v := testStruct{Name: "test", Age: 30}
	raw := marshalJSON(v)
	assert.NotEmpty(t, raw)

	var result testStruct
	err := unmarshalJSON(raw, &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, 30, result.Age)

	err = unmarshalJSON([]byte{}, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty json data")
}

func TestWrapHelpers(t *testing.T) {
	t.Run("wrapNotFound with pgx error", func(t *testing.T) {
		err := wrapNotFound(pgx.ErrNoRows, "entity", "test-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "entity with id test-id not found")
	})

	t.Run("wrapNotFound without pgx error", func(t *testing.T) {
		err := wrapNotFound(errors.New("some error"), "entity", "test-id")
		assert.Equal(t, "some error", err.Error())
	})

	t.Run("wrapSaveError", func(t *testing.T) {
		err := wrapSaveError(errors.New("db error"), "mission")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "save mission")
	})

	t.Run("wrapUpdateError", func(t *testing.T) {
		err := wrapUpdateError(errors.New("db error"), "plan")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update plan")
	})

	t.Run("wrapDeleteError", func(t *testing.T) {
		err := wrapDeleteError(errors.New("db error"), "session")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "delete session")
	})
}
