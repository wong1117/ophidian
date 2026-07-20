package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ophidian/ophidian/internal/infrastructure/dephealth"
)

func main() {
	checker := dephealth.NewChecker()
	fmt.Println("Checking dependency health...")
	report, err := checker.Check()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(report.Format())

	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile("dep-health-report.json", data, 0644)
	fmt.Println("Report saved to dep-health-report.json")

	if report.Status == "CRITICAL" {
		os.Exit(1)
	}
}
