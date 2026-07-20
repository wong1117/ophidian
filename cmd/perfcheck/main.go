package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ophidian/ophidian/internal/infrastructure/perf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: perfcheck <command> [args]")
		fmt.Println("  parse <bench-output-file>     Parse benchmark output to JSON")
		fmt.Println("  detect <baseline.json> <current.json>   Detect regressions")
		fmt.Println("  trend <history.json> <name>   Show trend for a benchmark")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "parse":
		if len(os.Args) < 3 {
			fmt.Println("Usage: perfcheck parse <bench-output-file>")
			os.Exit(1)
		}
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		results := perf.ParseBenchOutput(string(data), os.Args[2], "current", "go1.22", "linux", "amd64")
		json.NewEncoder(os.Stdout).Encode(results)

	case "detect":
		if len(os.Args) < 4 {
			fmt.Println("Usage: perfcheck detect <baseline.json> <current.json>")
			os.Exit(1)
		}
		baseline := loadResults(os.Args[2])
		current := loadResults(os.Args[3])

		history := perf.NewBenchmarkHistory("")
		for _, b := range baseline {
			history.Append([]perf.BenchmarkResult{b})
		}

		detector := perf.NewRegressionDetector(history, perf.DefaultRegressionConfig())
		report := detector.Report(current)
		fmt.Print(report.BenchstatFormat())

		data, _ := json.MarshalIndent(report, "", "  ")
		os.WriteFile("perf-regression-report.json", data, 0644)

		if !report.Passed {
			fmt.Println("\nREGRESSION DETECTED")
			os.Exit(1)
		}

	case "trend":
		if len(os.Args) < 4 {
			fmt.Println("Usage: perfcheck trend <history.json> <benchmark-name>")
			os.Exit(1)
		}
		history := perf.NewBenchmarkHistory(os.Args[2])
		detector := perf.NewRegressionDetector(history, perf.DefaultRegressionConfig())
		fmt.Print(detector.TrendReport(os.Args[3], 10))

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func loadResults(path string) []perf.BenchmarkResult {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	var results []perf.BenchmarkResult
	json.Unmarshal(data, &results)
	return results
}
