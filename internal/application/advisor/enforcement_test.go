package advisor

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnforcementEngine_Evaluate_Allowed(t *testing.T) {
	cfg := DefaultRoEConfig()
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:     "http_get",
		TargetIP:   "10.0.0.100",
		TargetPort: 80,
		Severity:   "LOW",
		CVSS:       2.0,
		Confidence: 0.9,
		RiskLevel:  1,
	}

	result := engine.Evaluate(req)

	assert.True(t, result.Allowed)
	assert.Equal(t, DecisionAllowed, result.Decision)
	assert.Empty(t, result.Violations)
}

func TestEnforcementEngine_Evaluate_DeniedNetwork(t *testing.T) {
	cfg := DefaultRoEConfig()
	cfg.Boundaries.DenyNetworks = []string{"10.0.0.0/24"}
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:   "scanner",
		TargetIP: "10.0.0.100",
	}

	result := engine.Evaluate(req)
	assert.False(t, result.Allowed)
	assert.Equal(t, DecisionDenied, result.Decision)
	assert.Len(t, result.Violations, 1)
	assert.Equal(t, "DENY_NETWORK", result.Violations[0].Rule)
}

func TestEnforcementEngine_Evaluate_AllowedNetwork(t *testing.T) {
	cfg := DefaultRoEConfig()
	cfg.Boundaries.AllowedNetworks = []string{"10.0.0.0/8"}
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:   "scanner",
		TargetIP: "10.0.0.1",
	}

	result := engine.Evaluate(req)
	assert.True(t, result.Allowed)
}

func TestEnforcementEngine_Evaluate_NotInAllowedNetwork(t *testing.T) {
	cfg := DefaultRoEConfig()
	cfg.Boundaries.AllowedNetworks = []string{"192.168.0.0/16"}
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:   "scanner",
		TargetIP: "8.8.8.8",
	}

	result := engine.Evaluate(req)
	assert.False(t, result.Allowed)
	assert.Equal(t, "BOUNDARY", result.Violations[0].Rule)
}

func TestEnforcementEngine_Evaluate_InvalidIP(t *testing.T) {
	cfg := DefaultRoEConfig()
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:   "scanner",
		TargetIP: "not-an-ip",
	}

	result := engine.Evaluate(req)
	assert.False(t, result.Allowed)
	assert.Equal(t, "BOUNDARY", result.Violations[0].Rule)
}

func TestEnforcementEngine_Evaluate_RiskSeverity(t *testing.T) {
	cfg := DefaultRoEConfig()
	cfg.Risk.MaxSeverity = "MEDIUM"
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:   "exploit",
		TargetIP: "10.0.0.1",
		Severity: "CRITICAL",
		CVSS:     10.0,
	}

	result := engine.Evaluate(req)
	assert.False(t, result.Allowed)
}

func TestEnforcementEngine_Evaluate_DestructiveDenied(t *testing.T) {
	cfg := DefaultRoEConfig()
	cfg.Risk.AllowDestructive = false
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:        "wipe",
		TargetIP:      "10.0.0.1",
		IsDestructive: true,
	}

	result := engine.Evaluate(req)
	assert.False(t, result.Allowed)
	assert.Equal(t, "RISK_DESTRUCTIVE", result.Violations[0].Rule)
}

func TestEnforcementEngine_Evaluate_AutoApprove(t *testing.T) {
	cfg := DefaultRoEConfig()
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:     "http_get",
		TargetIP:   "10.0.0.1",
		TargetPort: 80,
		RiskLevel:  1,
		Confidence: 0.9,
	}

	result := engine.Evaluate(req)
	assert.True(t, result.Allowed)
	assert.Greater(t, len(result.ApprovedActions), 0)
}

func TestEnforcementEngine_Evaluate_RequiresApproval(t *testing.T) {
	cfg := DefaultRoEConfig()
	cfg.Approval.RequireApproval = true
	cfg.Approval.MinConfidenceThreshold = 0.9
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:     "exploit",
		TargetIP:   "10.0.0.1",
		Confidence: 0.5,
		RiskLevel:  3,
	}

	result := engine.Evaluate(req)
	assert.True(t, result.RequiresApproval)
	assert.Equal(t, DecisionEscalated, result.Decision)
}

func TestEnforcementEngine_CalculateAIConfidence(t *testing.T) {
	cfg := DefaultRoEConfig()
	engine := NewEnforcementEngine(cfg)

	score := engine.CalculateAIConfidence(nil, 0.95, 0.8, true)
	assert.Greater(t, score, 0.7)

	scoreLow := engine.CalculateAIConfidence(nil, 0.4, 0.5, false)
	assert.Less(t, scoreLow, 0.4)
}

func TestEnforcementEngine_MustAbort(t *testing.T) {
	cfg := DefaultRoEConfig()
	cfg.Boundaries.DenyNetworks = []string{"10.0.0.0/24"}
	engine := NewEnforcementEngine(cfg)

	req := &ActionRequest{
		Action:   "scan",
		TargetIP: "10.0.0.100",
	}

	abort, reason := engine.MustAbort(req)
	assert.True(t, abort)
	assert.Contains(t, reason, "critical violation")
}

func TestConfigLoader_LoadDefaults(t *testing.T) {
	loader := NewConfigLoader("")
	cfg, err := loader.Load()
	assert.NoError(t, err)
	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, 3, cfg.Mission.MaxScanDepth)
}

func TestConfigLoader_LoadFromFile(t *testing.T) {
	content := `
version: "2.0"
mission:
  max_scan_depth: 5
  max_concurrent_ops: 10
boundaries:
  allowed_networks:
    - "10.0.0.0/8"
    - "172.16.0.0/12"
approval:
  require_approval: true
  min_confidence_threshold: 0.8
risk:
  max_severity: "MEDIUM"
  allow_destructive: false
`
	tmpFile := "/tmp/test-roe-config.yaml"
	os.WriteFile(tmpFile, []byte(content), 0644)
	defer os.Remove(tmpFile)

	loader := NewConfigLoader(tmpFile)
	cfg, err := loader.Load()

	assert.NoError(t, err)
	assert.Equal(t, "2.0", cfg.Version)
	assert.Equal(t, 5, cfg.Mission.MaxScanDepth)
	assert.True(t, cfg.Approval.RequireApproval)
}

func TestDefaultRoEConfig(t *testing.T) {
	cfg := DefaultRoEConfig()
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.Boundaries.AllowedNetworks)
	assert.NotEmpty(t, cfg.Approval.AutoApproveActions)
}
