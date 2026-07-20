package dashboard

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
	"github.com/stretchr/testify/assert"
)

type testMissionRepo struct {
	missions []*mission.Mission
}

func (r *testMissionRepo) Save(ctx context.Context, m *mission.Mission) error          { return nil }
func (r *testMissionRepo) FindByID(ctx context.Context, id string) (*mission.Mission, error) { return nil, nil }
func (r *testMissionRepo) FindAll(ctx context.Context, filter mission.MissionFilter) ([]*mission.Mission, error) {
	return r.missions, nil
}
func (r *testMissionRepo) Update(ctx context.Context, m *mission.Mission) error        { return nil }
func (r *testMissionRepo) Delete(ctx context.Context, id string) error                  { return nil }
func (r *testMissionRepo) SaveTask(ctx context.Context, task *mission.Task) error       { return nil }
func (r *testMissionRepo) FindTaskByID(ctx context.Context, id string) (*mission.Task, error) { return nil, nil }
func (r *testMissionRepo) FindTasksByMission(ctx context.Context, missionID string) ([]*mission.Task, error) {
	return nil, nil
}
func (r *testMissionRepo) UpdateTask(ctx context.Context, task *mission.Task) error     { return nil }

type testFindingRepo struct {
	findings []*finding.Finding
}

func (r *testFindingRepo) Save(ctx context.Context, f *finding.Finding) error            { return nil }
func (r *testFindingRepo) FindByID(ctx context.Context, id string) (*finding.Finding, error) { return nil, nil }
func (r *testFindingRepo) FindByMission(ctx context.Context, missionID string) ([]*finding.Finding, error) {
	return r.findings, nil
}
func (r *testFindingRepo) FindByTarget(ctx context.Context, targetID string) ([]*finding.Finding, error) {
	return nil, nil
}
func (r *testFindingRepo) Update(ctx context.Context, f *finding.Finding) error          { return nil }
func (r *testFindingRepo) Delete(ctx context.Context, id string) error                    { return nil }
func (r *testFindingRepo) SaveEvidence(ctx context.Context, ev *finding.Evidence) error   { return nil }
func (r *testFindingRepo) FindEvidenceByFinding(ctx context.Context, findingID string) ([]*finding.Evidence, error) {
	return nil, nil
}

type testMetrics struct {
	mu         sync.Mutex
	requests   int64
	errors     int64
	avgLatency float64
}

func (m *testMetrics) Snapshot() (int64, int64, float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests, m.errors, m.avgLatency
}

func (m *testMetrics) record(req, err int64, lat float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests += req
	m.errors += err
	m.avgLatency = lat
}

type testCache struct {
	mu   sync.Mutex
	data map[string]interface{}
}

func newTestCache() *testCache {
	return &testCache{data: make(map[string]interface{})}
}

func (c *testCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *testCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func TestDashboardService_GetOverview(t *testing.T) {
	mr := &testMissionRepo{
		missions: []*mission.Mission{
			{ID: common.NewID(), Status: mission.MissionActive, CreatedAt: common.Now(), UpdatedAt: common.Now()},
			{ID: common.NewID(), Status: mission.MissionCompleted, CreatedAt: common.Now(), UpdatedAt: common.Now()},
			{ID: common.NewID(), Status: mission.MissionFailed, CreatedAt: common.Now(), UpdatedAt: common.Now()},
		},
	}
	fr := &testFindingRepo{
		findings: []*finding.Finding{
			{ID: common.NewID(), Severity: common.SeverityCritical, CVSS: 9.5},
			{ID: common.NewID(), Severity: common.SeverityHigh, CVSS: 7.5},
			{ID: common.NewID(), Severity: common.SeverityMedium, CVSS: 5.0},
		},
	}
	m := &testMetrics{requests: 100, errors: 5, avgLatency: 42.5}
	cache := newTestCache()

	svc := NewDashboardService(mr, fr, m, cache)
	overview, err := svc.GetOverview(context.Background(), dto.DashboardFilter{})

	assert.NoError(t, err)
	assert.Equal(t, 3, overview.Missions.Total)
	assert.Equal(t, 1, overview.Missions.Active)
	assert.Equal(t, 1, overview.Missions.Completed)
	assert.Equal(t, 1, overview.Missions.Failed)
	assert.InDelta(t, 1.0/3.0, overview.Missions.SuccessRate, 0.01)

	assert.Equal(t, 3, overview.Findings.Total)
	assert.Equal(t, 1, overview.Findings.Critical)
	assert.Equal(t, 1, overview.Findings.High)
	assert.Equal(t, 1, overview.Findings.Medium)
	assert.InDelta(t, 7.33, overview.Findings.AvgCVSS, 0.1)

	assert.Equal(t, int64(100), overview.System.HTTPRequests)
	assert.Equal(t, int64(5), overview.System.HTTPErrors)
	assert.InDelta(t, 42.5, overview.System.AvgResponseTimeMs, 0.01)
	assert.NotEmpty(t, overview.System.Uptime)
	assert.Greater(t, overview.System.Goroutines, 0)
	assert.Greater(t, overview.System.MemoryUsage, uint64(0))

	cached, ok := cache.Get("dashboard:overview")
	assert.True(t, ok)
	assert.NotNil(t, cached)
}

func TestDashboardService_GetOverview_CacheHit(t *testing.T) {
	mr := &testMissionRepo{}
	fr := &testFindingRepo{}
	svc := NewDashboardService(mr, fr, nil, newTestCache())

	ctx := context.Background()
	_, _ = svc.GetOverview(ctx, dto.DashboardFilter{})

	overview2, err := svc.GetOverview(ctx, dto.DashboardFilter{})
	assert.NoError(t, err)
	assert.NotNil(t, overview2)
}

func TestDashboardService_GetTimeline(t *testing.T) {
	mr := &testMissionRepo{}
	fr := &testFindingRepo{}
	svc := NewDashboardService(mr, fr, nil, nil)

	timeline, err := svc.GetTimeline(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Equal(t, 0, timeline.Total)
	assert.Equal(t, 1, timeline.Page)
	assert.Equal(t, 10, timeline.PerPage)
}

func TestDashboardService_EmptyData(t *testing.T) {
	mr := &testMissionRepo{}
	fr := &testFindingRepo{}
	svc := NewDashboardService(mr, fr, nil, nil)

	overview, err := svc.GetOverview(context.Background(), dto.DashboardFilter{})

	assert.NoError(t, err)
	assert.Equal(t, 0, overview.Missions.Total)
	assert.Equal(t, 0, overview.Findings.Total)
}
