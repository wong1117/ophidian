package observability

import (
	"fmt"
	"math"
	"strings"

	"gopkg.in/yaml.v3"
)

type SLOConfig struct {
	Name        string  `yaml:"name"`
	Target      float64 `yaml:"target"`
	Window      string  `yaml:"window"`
	Description string  `yaml:"description"`
}

type SLOCalculator struct {
	configs []SLOConfig
}

func NewSLOCalculator() *SLOCalculator {
	return &SLOCalculator{
		configs: []SLOConfig{
			{Name: "Control Plane API Availability", Target: 99.9, Window: "30d", Description: "HTTP 5xx error rate <0.1%"},
			{Name: "Execution Plane Task Completion", Target: 99.5, Window: "7d", Description: "Task completion rate >99.5%"},
			{Name: "AI Plane Response Time", Target: 95.0, Window: "1d", Description: "95% AI requests <30s"},
			{Name: "EventStore Write Availability", Target: 99.99, Window: "30d", Description: "Append success >99.99%"},
		},
	}
}

type SLOStatus struct {
	Name           string  `json:"name"`
	Target         float64 `json:"target"`
	Current        float64 `json:"current"`
	ErrorBudget    float64 `json:"error_budget_pct"`
	RemainingBudget float64 `json:"remaining_budget_pct"`
	Status         string  `json:"status"`
}

func (s *SLOCalculator) Calculate(successRate float64) []SLOStatus {
	var results []SLOStatus
	for _, cfg := range s.configs {
		errorBudget := 100.0 - cfg.Target
		currentError := 100.0 - successRate
		remaining := errorBudget - currentError

		status := "HEALTHY"
		if remaining < 0 {
			status = "BREACHED"
		} else if remaining < errorBudget*0.25 {
			status = "WARNING"
		}

		results = append(results, SLOStatus{
			Name:            cfg.Name,
			Target:          cfg.Target,
			Current:         math.Round(successRate*100) / 100,
			ErrorBudget:     math.Round(errorBudget*100) / 100,
			RemainingBudget: math.Round(math.Max(0, remaining)*100) / 100,
			Status:          status,
		})
	}
	return results
}

func (s *SLOCalculator) FormatDashboard(successRate float64) string {
	results := s.Calculate(successRate)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Ophidian SLO Dashboard\n\n"))
	sb.WriteString(fmt.Sprintf("**Current Success Rate:** %.2f%%\n\n", successRate))

	sb.WriteString("| SLO | Target | Current | Error Budget | Remaining | Status |\n")
	sb.WriteString("|-----|--------|---------|-------------|-----------|--------|\n")

	for _, r := range results {
		icon := "🟢"
		if r.Status == "WARNING" {
			icon = "🟡"
		} else if r.Status == "BREACHED" {
			icon = "🔴"
		}
		sb.WriteString(fmt.Sprintf("| %s | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %s %s |\n",
			r.Name, r.Target, r.Current, r.ErrorBudget, r.RemainingBudget, icon, r.Status))
	}
	return sb.String()
}

type AlertValidator struct {
	rules []byte
}

func NewAlertValidator(rulesYAML []byte) *AlertValidator {
	return &AlertValidator{rules: rulesYAML}
}

type AlertValidationReport struct {
	Total   int      `json:"total"`
	Valid   int      `json:"valid"`
	Invalid int      `json:"invalid"`
	Errors  []string `json:"errors"`
}

func (r *AlertValidationReport) Format() string {
	var sb strings.Builder
	sb.WriteString("Alert Validation Report\n")
	sb.WriteString(fmt.Sprintf("Total: %d | Valid: %d | Invalid: %d\n", r.Total, r.Valid, r.Invalid))
	for _, e := range r.Errors {
		sb.WriteString(fmt.Sprintf("  - %s\n", e))
	}
	return sb.String()
}

func (v *AlertValidator) Validate() *AlertValidationReport {
	report := &AlertValidationReport{}
	var data map[string]interface{}
	if err := yaml.Unmarshal(v.rules, &data); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("YAML parse error: %v", err))
		report.Invalid++
		return report
	}

	groups, ok := data["groups"].([]interface{})
	if !ok {
		report.Errors = append(report.Errors, "no alert groups found")
		report.Invalid++
		return report
	}

	for _, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok { continue }
		rules, ok := group["rules"].([]interface{})
		if !ok { continue }

		for _, r := range rules {
			report.Total++
			rule, ok := r.(map[string]interface{})
			if !ok {
				report.Errors = append(report.Errors, fmt.Sprintf("invalid rule in group %v", group["name"]))
				report.Invalid++
				continue
			}

			alertName, hasName := rule["alert"].(string)
			if !hasName || alertName == "" {
				report.Errors = append(report.Errors, "rule missing alert name")
				report.Invalid++
				continue
			}

			expr, hasExpr := rule["expr"].(string)
			if !hasExpr || expr == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("rule %s missing expression", alertName))
				report.Invalid++
				continue
			}

			if strings.Contains(expr, "rate(") && !strings.Contains(expr, "[") {
				report.Errors = append(report.Errors, fmt.Sprintf("rule %s: rate() requires time range", alertName))
				report.Invalid++
				continue
			}

			report.Valid++
		}
	}

	return report
}
