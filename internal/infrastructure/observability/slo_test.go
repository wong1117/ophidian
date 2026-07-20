package observability

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSLOCalculator_Calculate(t *testing.T) {
	calc := NewSLOCalculator()

	t.Run("healthy", func(t *testing.T) {
		results := calc.Calculate(99.95)
		assert.Len(t, results, 4)
		for _, r := range results {
			assert.NotEmpty(t, r.Name)
			assert.Greater(t, r.Target, 0.0)
			assert.Equal(t, "HEALTHY", results[0].Status)
			assert.Equal(t, "HEALTHY", results[1].Status)
		}
	})

	t.Run("breached", func(t *testing.T) {
		results := calc.Calculate(85.0)
		assert.Equal(t, "BREACHED", results[0].Status)
	})

	t.Run("format", func(t *testing.T) {
		dashboard := calc.FormatDashboard(99.95)
		assert.Contains(t, dashboard, "Ophidian SLO Dashboard")
		assert.Contains(t, dashboard, "99.90%")
	})
}

func TestAlertValidator_Validate(t *testing.T) {
	t.Run("valid rules", func(t *testing.T) {
		rules := []byte(`
groups:
  - name: test
    rules:
      - alert: TestAlert
        expr: rate(http_requests_total[5m]) > 0
        for: 5m
        labels:
          severity: warning
`)
		v := NewAlertValidator(rules)
		report := v.Validate()
		assert.Equal(t, 1, report.Total)
		assert.Equal(t, 1, report.Valid)
		assert.Equal(t, 0, report.Invalid)

		_ = report.Format()
	})

	t.Run("invalid expr", func(t *testing.T) {
		rules := []byte(`
groups:
  - name: test
    rules:
      - alert: BadAlert
        expr: rate(http_requests_total) > 0
`)
		v := NewAlertValidator(rules)
		report := v.Validate()
		assert.Equal(t, 1, report.Invalid)
	})

	t.Run("missing alert name", func(t *testing.T) {
		rules := []byte(`
groups:
  - name: test
    rules:
      - expr: "1 > 0"
`)
		v := NewAlertValidator(rules)
		report := v.Validate()
		assert.Equal(t, 1, report.Invalid)
	})
}

func TestAlertValidationReport_Format(t *testing.T) {
	v := NewAlertValidator([]byte(`groups: [{name: t, rules: [{alert: ok, expr: "rate(x[5m]) > 0"}]}]`))
	report := v.Validate()
	s := report.Format()
	assert.Contains(t, s, "Alert Validation")
	assert.Contains(t, s, "Valid: 1")
}

func TestSLO_RealConfig(t *testing.T) {
	data, err := os.ReadFile("../../deploy/prometheus/observability.yaml")
	if err != nil {
		t.Skip("config file not found")
	}
	v := NewAlertValidator(data)
	report := v.Validate()
	t.Logf("Alert validation: %d total, %d valid, %d invalid", report.Total, report.Valid, report.Invalid)
	assert.Greater(t, report.Total, 0)
}
