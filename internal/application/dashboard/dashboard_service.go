package dashboard

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/domain/finding"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type MetricProvider interface {
	Snapshot() (requests int64, errors int64, avgLatency float64)
}

type CacheProvider interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
}

type DashboardService struct {
	missionRepo mission.MissionRepository
	findingRepo finding.FindingRepository
	metrics     MetricProvider
	cache       CacheProvider
	startTime   time.Time
}

func NewDashboardService(
	missionRepo mission.MissionRepository,
	findingRepo finding.FindingRepository,
	metrics MetricProvider,
	cache CacheProvider,
) *DashboardService {
	return &DashboardService{
		missionRepo: missionRepo,
		findingRepo: findingRepo,
		metrics:     metrics,
		cache:       cache,
		startTime:   time.Now(),
	}
}

func (s *DashboardService) GetOverview(ctx context.Context, filter dto.DashboardFilter) (*dto.DashboardOverview, error) {
	cacheKey := fmt.Sprintf("dashboard:overview:tenant=%s", filter.TenantID)
	if filter.TenantID == "" {
		cacheKey = "dashboard:overview"
	}
	if s.cache != nil {
		if cached, ok := s.cache.Get(cacheKey); ok {
			if overview, ok := cached.(*dto.DashboardOverview); ok {
				return overview, nil
			}
		}
	}

	var (
		missionStats dto.MissionStats
		findingStats dto.FindingStats
		wg           sync.WaitGroup
		errs         []error
		mu           sync.Mutex
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		ms, err := s.aggregateMissionStats(ctx, filter)
		mu.Lock()
		if err != nil {
			errs = append(errs, err)
		} else {
			missionStats = ms
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		fs, err := s.aggregateFindingStats(ctx, filter)
		mu.Lock()
		if err != nil {
			errs = append(errs, err)
		} else {
			findingStats = fs
		}
		mu.Unlock()
	}()

	wg.Wait()

	overview := &dto.DashboardOverview{
		Missions:    missionStats,
		Findings:    findingStats,
		Workflows:   dto.WorkflowStats{},
		Queues:      dto.QueueStats{},
		Workers:     dto.WorkerStats{},
		System:      s.aggregateSystemMetrics(),
		GeneratedAt: time.Now().UTC(),
	}

	if s.cache != nil {
		s.cache.Set(cacheKey, overview, 30*time.Second)
	}

	return overview, nil
}

func (s *DashboardService) aggregateMissionStats(ctx context.Context, filter dto.DashboardFilter) (dto.MissionStats, error) {
	missions, err := s.missionRepo.FindAll(ctx, mission.MissionFilter{})
	if err != nil {
		return dto.MissionStats{}, err
	}

	stats := dto.MissionStats{Total: len(missions)}
	var totalDuration int64
	for _, m := range missions {
		switch m.Status {
		case mission.MissionActive:
			stats.Active++
		case mission.MissionCompleted:
			stats.Completed++
		case mission.MissionFailed:
			stats.Failed++
		case mission.MissionPaused:
			stats.Paused++
		}
		dur := m.UpdatedAt.Sub(m.CreatedAt)
		totalDuration += dur.Milliseconds()
	}
	if stats.Total > 0 {
		stats.SuccessRate = float64(stats.Completed) / float64(stats.Total)
		stats.AvgDuration = totalDuration / int64(stats.Total)
	}
	return stats, nil
}

func (s *DashboardService) aggregateFindingStats(ctx context.Context, filter dto.DashboardFilter) (dto.FindingStats, error) {
	findings, err := s.findingRepo.FindByMission(ctx, "")
	if err != nil {
		return dto.FindingStats{}, err
	}

	stats := dto.FindingStats{Total: len(findings)}
	var totalCVSS float64
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			stats.Critical++
		case "HIGH":
			stats.High++
		case "MEDIUM":
			stats.Medium++
		case "LOW":
			stats.Low++
		}
		totalCVSS += f.CVSS
	}
	if stats.Total > 0 {
		stats.AvgCVSS = totalCVSS / float64(stats.Total)
	}
	return stats, nil
}

func (s *DashboardService) aggregateSystemMetrics() dto.SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	requests, errors, avgLatency := int64(0), int64(0), float64(0)
	if s.metrics != nil {
		requests, errors, avgLatency = s.metrics.Snapshot()
	}

	return dto.SystemMetrics{
		Uptime:            time.Since(s.startTime).String(),
		Goroutines:        runtime.NumGoroutine(),
		MemoryUsage:       memStats.Alloc,
		HTTPRequests:      requests,
		HTTPErrors:        errors,
		AvgResponseTimeMs: avgLatency,
	}
}

func (s *DashboardService) GetTimeline(ctx context.Context, page, perPage int) (*dto.DashboardTimeline, error) {
	entries := make([]dto.TimelineEntry, 0)
	total := len(entries)

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	return &dto.DashboardTimeline{
		Entries: entries,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}
