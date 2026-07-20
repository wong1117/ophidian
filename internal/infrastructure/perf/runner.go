package perf

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Runner struct {
	packages []string
	history  *BenchmarkHistory
}

func NewRunner(history *BenchmarkHistory, packages ...string) *Runner {
	if len(packages) == 0 {
		packages = []string{
			"./internal/infrastructure/persistence/postgres/",
			"./internal/infrastructure/scheduler/",
			"./internal/infrastructure/queue/",
			"./internal/infrastructure/worker/",
			"./internal/infrastructure/workflow/",
			"./internal/infrastructure/persistence/redis/",
			"./internal/infrastructure/ai/router/",
			"./internal/infrastructure/memory/",
			"./pkg/exploit/",
			"./pkg/executor/",
			"./internal/infrastructure/config/",
			"./internal/infrastructure/secrets/",
			"./internal/application/recommendation/",
			"./internal/application/policy/",
		}
	}
	return &Runner{packages: packages, history: history}
}

func (r *Runner) RunAll() ([]BenchmarkResult, error) {
	var allResults []BenchmarkResult
	commit := getCommit()
	goVersion := getGoVersion()
	hostOS := getOS()
	hostArch := getArch()

	for _, pkg := range r.packages {
		results, err := r.runPackage(pkg, commit, goVersion, hostOS, hostArch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: benchmark %s failed: %v\n", pkg, err)
			continue
		}
		allResults = append(allResults, results...)
	}

	if len(allResults) > 0 {
		r.history.Append(allResults)
	}

	return allResults, nil
}

func (r *Runner) runPackage(pkg, commit, goVersion, hostOS, hostArch string) ([]BenchmarkResult, error) {
	cmd := exec.Command("go", "test", "-bench=.", "-benchmem", "-benchtime=1s", "-count=3", pkg, "-run=^$")
	cmd.Env = append(os.Environ(), "GOGC=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("benchmark %s: %w\n%s", pkg, err, string(output))
	}

	return ParseBenchOutput(string(output), pkg, commit, goVersion, hostOS, hostArch), nil
}

func ParseBenchOutput(output, pkg, commit, goVersion, hostOS, hostArch string) []BenchmarkResult {
	var results []BenchmarkResult
	now := time.Now()
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		name := parts[0]
		name = strings.TrimSuffix(name, "-2")
		name = strings.TrimSuffix(name, "-4")
		name = strings.TrimSuffix(name, "-8")

		opsPerSec := parseFloat(parts[1])
		nsPerOp := parseFloat(parts[2])
		bytesPerOp := int64(0)
		allocsPerOp := int64(0)

		for i, p := range parts {
			if p == "B/op" && i > 0 {
				bytesPerOp = parseInt(parts[i-1])
			}
			if p == "allocs/op" && i > 0 {
				allocsPerOp = parseInt(parts[i-1])
			}
		}

		results = append(results, BenchmarkResult{
			Name:       name,
			OpsPerSec:  opsPerSec,
			NSPerOp:    nsPerOp,
			BytesPerOp: bytesPerOp,
			AllocsPerOp: allocsPerOp,
			Timestamp:  now,
			Commit:     commit,
			Package:    pkg,
			GoVersion:  goVersion,
			OS:         hostOS,
			Arch:       hostArch,
		})
	}

	return results
}

func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(strings.TrimSpace(s), "%f", &v)
	return v
}

func parseInt(s string) int64 {
	var v int64
	fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v
}

func getCommit() string {
	if data, err := os.ReadFile(".git/HEAD"); err == nil {
		return strings.TrimSpace(string(data))
	}
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return "unknown"
}

func getGoVersion() string {
	out, err := exec.Command("go", "version").Output()
	if err != nil {
		return "unknown"
	}
	parts := strings.Fields(string(out))
	if len(parts) >= 3 {
		return parts[2]
	}
	return "unknown"
}

func getOS() string {
	return os.Getenv("GOOS")
}

func getArch() string {
	return os.Getenv("GOARCH")
}

type CLI struct {
	history  *BenchmarkHistory
	runner   *Runner
}

func NewCLI(historyPath string) *CLI {
	history := NewBenchmarkHistory(historyPath)
	return &CLI{
		history: history,
		runner:  NewRunner(history),
	}
}

func (c *CLI) Run() {
	fmt.Println("Running performance benchmarks...")
	results, err := c.runner.RunAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	detector := NewRegressionDetector(c.history, DefaultRegressionConfig())
	report := detector.Report(results)
	fmt.Print(report.BenchstatFormat())

	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile("bench-results.json", data, 0644)

	if !report.Passed {
		os.Exit(1)
	}
}

func (c *CLI) Trend(name string, limit int) {
	detector := NewRegressionDetector(c.history, DefaultRegressionConfig())
	fmt.Print(detector.TrendReport(name, limit))
}
