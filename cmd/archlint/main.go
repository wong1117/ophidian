package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ophidian/ophidian/internal/infrastructure/archlint"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	checker := archlint.NewChecker()
	report := checker.CheckAll(root)
	fmt.Print(report.Format())

	if len(os.Args) > 2 && os.Args[2] == "--json" {
		data, _ := json.MarshalIndent(report, "", "  ")
		os.WriteFile("arch-compliance-report.json", data, 0644)
		fmt.Println("Report saved to arch-compliance-report.json")
	}

	if !report.Pass {
		os.Exit(1)
	}
}
