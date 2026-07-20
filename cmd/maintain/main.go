package main

import (
	"fmt"
	"os"

	"github.com/ophidian/ophidian/internal/infrastructure/maintain"
)

func main() {
	analyzer := maintain.NewAnalyzer(".")
	report := analyzer.Run()

	fmt.Print(report.Format())

	os.WriteFile("maintainability-report.json", report.ToJSON(), 0644)
	fmt.Println("\nReport saved to maintainability-report.json")

	if report.Score < 70 {
		os.Exit(1)
	}
}
