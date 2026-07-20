package perf

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseBenchOutput(t *testing.T) {
	output := `goos: linux
goarch: amd64
BenchmarkQueue_Enqueue-2   	  500000	      2500 ns/op	     500 B/op	       3 allocs/op
BenchmarkQueue_Dequeue-2   	  300000	      4000 ns/op	    1000 B/op	       5 allocs/op
`
	results := ParseBenchOutput(output, "test/pkg", "abc123", "go1.22", "linux", "amd64")

	assert.Len(t, results, 2)
	assert.Equal(t, "BenchmarkQueue_Enqueue", results[0].Name)
	assert.Equal(t, float64(2500), results[0].NSPerOp)
	assert.Equal(t, int64(500), results[0].BytesPerOp)
	assert.Equal(t, int64(3), results[0].AllocsPerOp)
	assert.Equal(t, "BenchmarkQueue_Dequeue", results[1].Name)
}

func TestRegressionDetector_Detect(t *testing.T) {
	tmpFile := "/tmp/test-bench-history.json"
	defer os.Remove(tmpFile)

	history := NewBenchmarkHistory(tmpFile)
	baseline := []BenchmarkResult{
		{Name: "TestFunc", NSPerOp: 1000, BytesPerOp: 256, Commit: "old", Timestamp: time.Now().Add(-time.Hour)},
		{Name: "TestFunc", NSPerOp: 1000, BytesPerOp: 256, Commit: "old", Timestamp: time.Now().Add(-time.Hour)},
	}
	history.Append(baseline)

	detector := NewRegressionDetector(history, RegressionConfig{
		MaxCPUIncreasePct:   5.0,
		MaxAllocIncreasePct: 10.0,
		MinBaselineRuns:     1,
	})

	t.Run("no regression", func(t *testing.T) {
		current := []BenchmarkResult{
			{Name: "TestFunc", NSPerOp: 1020, BytesPerOp: 260},
		}
		report := detector.Report(current)
		assert.True(t, report.Passed)
		assert.Equal(t, 0, report.RegressionCount)
	})

	t.Run("cpu regression", func(t *testing.T) {
		detector.baseline = baseline
		current := []BenchmarkResult{
			{Name: "TestFunc", NSPerOp: 1200, BytesPerOp: 260},
		}
		report := detector.Report(current)
		assert.False(t, report.Passed)
		assert.Equal(t, 1, report.RegressionCount)
	})
}

func TestRegressionDetector_NoBaseline(t *testing.T) {
	tmpFile := "/tmp/test-empty-history.json"
	defer os.Remove(tmpFile)

	history := NewBenchmarkHistory(tmpFile)
	detector := NewRegressionDetector(history, DefaultRegressionConfig())

	current := []BenchmarkResult{
		{Name: "NewTest", NSPerOp: 5000, BytesPerOp: 1024},
	}
	report := detector.Report(current)
	assert.True(t, report.Passed)
	assert.Equal(t, 0, report.RegressionCount)
}

func TestBenchmarkHistory_AppendAndLoad(t *testing.T) {
	tmpFile := "/tmp/test-history-save.json"
	defer os.Remove(tmpFile)

	h1 := NewBenchmarkHistory(tmpFile)
	h1.Append([]BenchmarkResult{
		{Name: "TestA", NSPerOp: 100, Package: "pkg/a", Commit: "c1"},
	})

	h2 := NewBenchmarkHistory(tmpFile)
	assert.Len(t, h2.entries, 1)
}

func TestBenchmarkHistory_Trend(t *testing.T) {
	tmpFile := "/tmp/test-trend.json"
	defer os.Remove(tmpFile)

	h := NewBenchmarkHistory(tmpFile)
	h.Append([]BenchmarkResult{
		{Name: "TrendTest", NSPerOp: 100, Commit: "c1", Timestamp: time.Now().Add(-3 * time.Hour)},
		{Name: "TrendTest", NSPerOp: 120, Commit: "c2", Timestamp: time.Now().Add(-2 * time.Hour)},
		{Name: "TrendTest", NSPerOp: 110, Commit: "c3", Timestamp: time.Now().Add(-1 * time.Hour)},
	})

	detector := NewRegressionDetector(h, DefaultRegressionConfig())
	report := detector.TrendReport("TrendTest", 10)
	assert.Contains(t, report, "TrendTest")
	assert.Contains(t, report, "100")
	assert.Contains(t, report, "120")
}

func TestRegressionReport_BenchstatFormat(t *testing.T) {
	report := &RegressionReport{
		GeneratedAt: time.Now(),
		Passed:      false,
		Results: []RegressionResult{
			{Name: "SlowFunc", Severity: "WARNING", BaselineNSPerOp: 1000, CurrentNSPerOp: 1200, CPUPctChange: 20.0, BaselineBytesPerOp: 256, CurrentBytesPerOp: 300, AllocPctChange: 17.0, Regressed: true},
		},
		RegressionCount: 1,
		WarningCount:    1,
		BaselineCount:   1,
		CurrentCount:    1,
	}

	reportStr := report.BenchstatFormat()
	assert.Contains(t, reportStr, "FAIL")
	assert.Contains(t, reportStr, "SlowFunc")
	assert.Contains(t, reportStr, "1000")
	assert.Contains(t, reportStr, "1200")
	assert.Contains(t, reportStr, "+20.0%")
}

func TestRegressionReport_JSON(t *testing.T) {
	report := &RegressionReport{
		Passed: true,
		Results: []RegressionResult{
			{Name: "FastFunc", Severity: "OK", Regressed: false},
		},
	}
	data := report.ToJSON()
	assert.True(t, json.Valid(data))
	assert.Contains(t, string(data), "FastFunc")
}

func TestCLI_ParseCommand(t *testing.T) {
	tmpFile := "/tmp/perf-test-input.txt"
	os.WriteFile(tmpFile, []byte("BenchmarkTest-2   1000000 1000 ns/op 100 B/op 2 allocs/op\n"), 0644)
	defer os.Remove(tmpFile)

	results := ParseBenchOutput(string(mustRead(tmpFile)), "pkg", "commit", "go1.22", "linux", "amd64")
	assert.Len(t, results, 1)
	assert.Equal(t, "BenchmarkTest", results[0].Name)
}

func mustRead(path string) []byte {
	d, _ := os.ReadFile(path)
	return d
}
