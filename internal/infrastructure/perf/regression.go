package perf

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type BenchmarkResult struct {
	Name       string  `json:"name"`
	OpsPerSec  float64 `json:"ops_per_sec"`
	NSPerOp    float64 `json:"ns_per_op"`
	BytesPerOp int64   `json:"bytes_per_op"`
	AllocsPerOp int64  `json:"allocs_per_op"`
	Timestamp  time.Time `json:"timestamp"`
	Commit     string  `json:"commit"`
	Package    string  `json:"package"`
	GoVersion  string  `json:"go_version"`
	OS         string  `json:"os"`
	Arch       string  `json:"arch"`
}

type BenchmarkHistory struct {
	mu       sync.RWMutex
	entries  []BenchmarkResult
	filePath string
}

func NewBenchmarkHistory(filePath string) *BenchmarkHistory {
	h := &BenchmarkHistory{filePath: filePath}
	h.Load()
	return h
}

func (h *BenchmarkHistory) Load() {
	data, err := os.ReadFile(h.filePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &h.entries)
}

func (h *BenchmarkHistory) Save() error {
	data, err := json.MarshalIndent(h.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("save benchmark history: %w", err)
	}
	return os.WriteFile(h.filePath, data, 0644)
}

func (h *BenchmarkHistory) Append(results []BenchmarkResult) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, results...)
	return h.Save()
}

func (h *BenchmarkHistory) Baseline(pkgFilter string) []BenchmarkResult {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.filter(pkgFilter)
}

func (h *BenchmarkHistory) Latest(pkgFilter string) []BenchmarkResult {
	h.mu.RLock()
	defer h.mu.RUnlock()
	entries := h.filter(pkgFilter)
	if len(entries) == 0 {
		return nil
	}

	latestCommit := entries[0].Commit
	for _, e := range entries {
		if e.Commit != latestCommit {
			break
		}
	}

	var latest []BenchmarkResult
	for _, e := range entries {
		if e.Commit == latestCommit {
			latest = append(latest, e)
		}
	}
	return latest
}

func (h *BenchmarkHistory) filter(pkgFilter string) []BenchmarkResult {
	if pkgFilter == "" {
		return h.entries
	}
	var filtered []BenchmarkResult
	for _, e := range h.entries {
		if strings.HasPrefix(e.Package, pkgFilter) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (h *BenchmarkHistory) Trend(name string, limit int) []TrendPoint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var points []BenchmarkResult
	for _, e := range h.entries {
		if e.Name == name {
			points = append(points, e)
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp.Before(points[j].Timestamp) })

	var trend []TrendPoint
	for _, p := range points {
		trend = append(trend, TrendPoint{
			Timestamp:  p.Timestamp,
			Commit:     p.Commit,
			NSPerOp:    p.NSPerOp,
			BytesPerOp: p.BytesPerOp,
			AllocsPerOp: p.AllocsPerOp,
		})
	}

	if limit > 0 && len(trend) > limit {
		trend = trend[len(trend)-limit:]
	}
	return trend
}

type TrendPoint struct {
	Timestamp    time.Time
	Commit       string
	NSPerOp      float64
	BytesPerOp   int64
	AllocsPerOp  int64
}

type RegressionConfig struct {
	MaxCPUIncreasePct   float64 `json:"max_cpu_increase_pct"`
	MaxAllocIncreasePct  float64 `json:"max_alloc_increase_pct"`
	MinBaselineRuns     int     `json:"min_baseline_runs"`
}

func DefaultRegressionConfig() RegressionConfig {
	return RegressionConfig{
		MaxCPUIncreasePct:   5.0,
		MaxAllocIncreasePct: 10.0,
		MinBaselineRuns:     3,
	}
}

type RegressionResult struct {
	Name            string  `json:"name"`
	BaselineNSPerOp     float64 `json:"baseline_ns_per_op"`
	CurrentNSPerOp      float64 `json:"current_ns_per_op"`
	CPUPctChange        float64 `json:"cpu_pct_change"`
	BaselineBytesPerOp  int64   `json:"baseline_bytes_per_op"`
	CurrentBytesPerOp   int64   `json:"current_bytes_per_op"`
	AllocPctChange      float64 `json:"alloc_pct_change"`
	Regressed           bool    `json:"regressed"`
	Severity            string  `json:"severity"`
}

type RegressionDetector struct {
	config    RegressionConfig
	history   *BenchmarkHistory
	baseline  []BenchmarkResult
}

func NewRegressionDetector(history *BenchmarkHistory, cfg RegressionConfig) *RegressionDetector {
	if cfg.MinBaselineRuns <= 0 {
		cfg = DefaultRegressionConfig()
	}
	return &RegressionDetector{
		config:   cfg,
		history:  history,
		baseline: history.Latest(""),
	}
}

func (d *RegressionDetector) Detect(current []BenchmarkResult) []RegressionResult {
	baselineMap := make(map[string]BenchmarkResult)
	for _, b := range d.baseline {
		if existing, ok := baselineMap[b.Name]; !ok || b.NSPerOp < existing.NSPerOp {
			baselineMap[b.Name] = b
		}
	}

	var results []RegressionResult
	for _, c := range current {
		b, ok := baselineMap[c.Name]
		if !ok {
			continue
		}

		cpuDelta := ((c.NSPerOp - b.NSPerOp) / b.NSPerOp) * 100
		allocDelta := float64(0)
		if b.BytesPerOp > 0 {
			allocDelta = (float64(c.BytesPerOp-b.BytesPerOp) / float64(b.BytesPerOp)) * 100
		}

		regressed := cpuDelta > d.config.MaxCPUIncreasePct || allocDelta > d.config.MaxAllocIncreasePct
		severity := "OK"
		if regressed {
			if cpuDelta > d.config.MaxCPUIncreasePct*3 {
				severity = "CRITICAL"
			} else if cpuDelta > d.config.MaxCPUIncreasePct*2 {
				severity = "HIGH"
			} else {
				severity = "WARNING"
			}
		}

		results = append(results, RegressionResult{
			Name:              c.Name,
			BaselineNSPerOp:   b.NSPerOp,
			CurrentNSPerOp:    c.NSPerOp,
			CPUPctChange:      cpuDelta,
			BaselineBytesPerOp: b.BytesPerOp,
			CurrentBytesPerOp: c.BytesPerOp,
			AllocPctChange:    allocDelta,
			Regressed:         regressed,
			Severity:          severity,
		})
	}

	return results
}

func (d *RegressionDetector) HasRegression(results []RegressionResult) bool {
	for _, r := range results {
		if r.Regressed {
			return true
		}
	}
	return false
}

func (d *RegressionDetector) Report(current []BenchmarkResult) *RegressionReport {
	results := d.Detect(current)

	report := &RegressionReport{
		GeneratedAt:  time.Now(),
		Config:       d.config,
		BaselineCount: len(d.baseline),
		CurrentCount:  len(current),
		Results:      results,
	}

	for _, r := range results {
		if r.Regressed {
			report.RegressionCount++
			if r.Severity == "CRITICAL" {
				report.CriticalCount++
			} else if r.Severity == "HIGH" {
				report.HighCount++
			} else {
				report.WarningCount++
			}
		}
	}

	report.Passed = report.RegressionCount == 0
	return report
}

type RegressionReport struct {
	GeneratedAt     time.Time          `json:"generated_at"`
	Config          RegressionConfig   `json:"config"`
	Passed          bool               `json:"passed"`
	BaselineCount   int                `json:"baseline_count"`
	CurrentCount    int                `json:"current_count"`
	RegressionCount int                `json:"regression_count"`
	CriticalCount   int                `json:"critical_count"`
	HighCount       int                `json:"high_count"`
	WarningCount    int                `json:"warning_count"`
	Results        []RegressionResult  `json:"results"`
}

func (r *RegressionReport) BenchstatFormat() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Performance Regression Report\n"))
	sb.WriteString(fmt.Sprintf("Generated: %s\n", r.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Baseline: %d runs | Current: %d runs\n\n", r.BaselineCount, r.CurrentCount))

	totalResults := len(r.Results)
	passCount := r.BaselineCount - r.RegressionCount

	if totalResults > 0 {
		sb.WriteString(fmt.Sprintf("Results: %d passed, %d regressed\n\n", passCount, r.RegressionCount))
	}

	for _, res := range r.Results {
		if !res.Regressed {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", res.Severity, res.Name))
		sb.WriteString(fmt.Sprintf("  CPU:  %.2f ns/op → %.2f ns/op (%+.1f%%)\n", res.BaselineNSPerOp, res.CurrentNSPerOp, res.CPUPctChange))
		if res.BaselineBytesPerOp > 0 {
			sb.WriteString(fmt.Sprintf("  Alloc: %d B/op → %d B/op (%+.1f%%)\n", res.BaselineBytesPerOp, res.CurrentBytesPerOp, res.AllocPctChange))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Summary: %s\n", r.Status()))
	return sb.String()
}

func (r *RegressionReport) Status() string {
	if r.Passed {
		return "PASS — No performance regressions detected"
	}
	return fmt.Sprintf("FAIL — %d regressions (%d critical, %d high, %d warning)",
		r.RegressionCount, r.CriticalCount, r.HighCount, r.WarningCount)
}

func (r *RegressionReport) ToJSON() []byte {
	data, _ := json.MarshalIndent(r, "", "  ")
	return data
}

func (d *RegressionDetector) TrendReport(name string, limit int) string {
	points := d.history.Trend(name, limit)
	if len(points) == 0 {
		return fmt.Sprintf("No trend data for %s\n", name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Performance Trend: %s\n", name))
	sb.WriteString(fmt.Sprintf("%-20s %-12s %-10s %-10s\n", "Timestamp", "Commit", "ns/op", "B/op"))
	sb.WriteString(strings.Repeat("-", 55) + "\n")

	for _, p := range points {
		commit := p.Commit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		sb.WriteString(fmt.Sprintf("%-20s %-12s %-10.0f %-10d\n",
			p.Timestamp.Format("2006-01-02 15:04"), commit, p.NSPerOp, p.BytesPerOp))
	}
	return sb.String()
}
