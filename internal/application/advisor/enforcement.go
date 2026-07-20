package advisor

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type RoEConfig struct {
	Version     string              `yaml:"version"`
	Mission     MissionScope        `yaml:"mission"`
	Boundaries  BoundaryRules       `yaml:"boundaries"`
	Approval    ApprovalRules       `yaml:"approval"`
	Risk        RiskLimits          `yaml:"risk"`
	AI          AIAssuranceConfig   `yaml:"ai"`
}

type MissionScope struct {
	MaxScanDepth     int      `yaml:"max_scan_depth"`
	MaxConcurrentOps int      `yaml:"max_concurrent_ops"`
	MaxSessionCount  int      `yaml:"max_session_count"`
	AllowedPorts     []int    `yaml:"allowed_ports"`
	ExcludedPorts    []int    `yaml:"excluded_ports"`
	TimeWindowStart  string   `yaml:"time_window_start"`
	TimeWindowEnd    string   `yaml:"time_window_end"`
}

type BoundaryRules struct {
	AllowedNetworks   []string `yaml:"allowed_networks"`
	DenyNetworks      []string `yaml:"deny_networks"`
	DenyDomains       []string `yaml:"deny_domains"`
	MaxHostsPerScope  int      `yaml:"max_hosts_per_scope"`
}

type ApprovalRules struct {
	RequireApproval       bool     `yaml:"require_approval"`
	MinConfidenceThreshold float64 `yaml:"min_confidence_threshold"`
	AutoApproveActions    []string `yaml:"auto_approve_actions"`
	AutoApprovePorts      []int    `yaml:"auto_approve_ports"`
	AutoApproveTargets    []string `yaml:"auto_approve_targets"`
	MaxAutoApproveRisk    int      `yaml:"max_auto_approve_risk"`
}

type RiskLimits struct {
	MaxSeverity        string  `yaml:"max_severity"`
	MaxCVSS            float64 `yaml:"max_cvss"`
	MaxExploitRisk     int     `yaml:"max_exploit_risk"`
	AllowDestructive   bool    `yaml:"allow_destructive"`
	AllowPersistence   bool    `yaml:"allow_persistence"`
	AllowExfiltration  bool    `yaml:"allow_exfiltration"`
}

type AIAssuranceConfig struct {
	Provider          string  `yaml:"provider"`
	Model             string  `yaml:"model"`
	MinPlanConfidence float64 `yaml:"min_plan_confidence"`
	MinTTPConfidence  float64 `yaml:"min_ttp_confidence"`
	RequireEvidence   bool    `yaml:"require_evidence"`
	RequireHumanReview bool   `yaml:"require_human_review"`
}

type EnforcementResult struct {
	Allowed          bool
	Decision         Decision
	Reason           string
	Confidence       float64
	Violations       []Violation
	RequiresApproval bool
	ApprovedActions  []string
}

type Decision string

const (
	DecisionAllowed   Decision = "ALLOWED"
	DecisionDenied    Decision = "DENIED"
	DecisionEscalated Decision = "ESCALATED"
)

type Violation struct {
	Rule    string
	Value   string
	Message string
}

type EnforcementEngine struct {
	mu     sync.RWMutex
	config *RoEConfig
}

type ConfigLoader struct {
	path string
}

func NewConfigLoader(path string) *ConfigLoader {
	return &ConfigLoader{path: path}
}

func (l *ConfigLoader) Load() (*RoEConfig, error) {
	cfg := DefaultRoEConfig()
	if l.path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("load roe config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse roe config: %w", err)
	}
	return cfg, nil
}

func DefaultRoEConfig() *RoEConfig {
	return &RoEConfig{
		Version: "1.0",
		Mission: MissionScope{
			MaxScanDepth:     3,
			MaxConcurrentOps: 5,
			MaxSessionCount:  10,
			AllowedPorts:     []int{80, 443, 22, 8080, 8443},
			ExcludedPorts:    []int{},
		},
		Boundaries: BoundaryRules{
			AllowedNetworks:  []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
			DenyNetworks:     []string{},
			MaxHostsPerScope: 100,
		},
		Approval: ApprovalRules{
			RequireApproval:       false,
			MinConfidenceThreshold: 0.7,
			AutoApproveActions:    []string{"ping", "whois", "dns_lookup", "banner_grab", "http_get"},
			AutoApprovePorts:      []int{80, 443},
			MaxAutoApproveRisk:    2,
		},
		Risk: RiskLimits{
			MaxSeverity:      "HIGH",
			MaxCVSS:          9.0,
			MaxExploitRisk:   3,
			AllowDestructive: false,
			AllowPersistence: false,
			AllowExfiltration: false,
		},
		AI: AIAssuranceConfig{
			Provider:           "openai",
			Model:              "gpt-4",
			MinPlanConfidence:  0.7,
			MinTTPConfidence:   0.6,
			RequireEvidence:    true,
			RequireHumanReview: false,
		},
	}
}

func NewEnforcementEngine(cfg *RoEConfig) *EnforcementEngine {
	return &EnforcementEngine{config: cfg}
}

type ActionRequest struct {
	Action     string
	TargetIP   string
	TargetPort int
	CVE        string
	CVSS       float64
	Severity   string
	Confidence float64
	Technique  string
	Evidence   bool
	IsDestructive   bool
	IsPersistence   bool
	IsExfiltration  bool
	RiskLevel  int
}

func (e *EnforcementEngine) Evaluate(req *ActionRequest) *EnforcementResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &EnforcementResult{
		Allowed:  true,
		Decision: DecisionAllowed,
	}

	e.checkBoundaries(req, result)
	e.checkRiskLimits(req, result)
	e.checkAutoApprove(req, result)
	e.checkApproval(req, result)

	if len(result.Violations) > 0 {
		result.Allowed = false
		result.Decision = DecisionDenied
	}

	return result
}

func (e *EnforcementEngine) checkBoundaries(req *ActionRequest, result *EnforcementResult) {
	ip := net.ParseIP(req.TargetIP)
	if ip == nil {
		result.Violations = append(result.Violations, Violation{
			Rule: "BOUNDARY", Value: req.TargetIP, Message: "Invalid IP address",
		})
		return
	}

	for _, deny := range e.config.Boundaries.DenyNetworks {
		_, denyNet, err := net.ParseCIDR(deny)
		if err != nil {
			continue
		}
		if denyNet.Contains(ip) {
			result.Violations = append(result.Violations, Violation{
				Rule: "DENY_NETWORK", Value: deny, Message: fmt.Sprintf("Target %s is in denied network %s", req.TargetIP, deny),
			})
			return
		}
	}

	allowed := false
	for _, allow := range e.config.Boundaries.AllowedNetworks {
		_, allowNet, err := net.ParseCIDR(allow)
		if err != nil {
			continue
		}
		if allowNet.Contains(ip) {
			allowed = true
			break
		}
	}
	if !allowed {
		result.Violations = append(result.Violations, Violation{
			Rule: "BOUNDARY", Value: req.TargetIP, Message: "Target not in any allowed network",
		})
	}
}

func (e *EnforcementEngine) checkRiskLimits(req *ActionRequest, result *EnforcementResult) {
	severityOrder := map[string]int{"CRITICAL": 5, "HIGH": 4, "MEDIUM": 3, "LOW": 2, "INFO": 1}
	reqSev := severityOrder[strings.ToUpper(req.Severity)]
	maxSev := severityOrder[strings.ToUpper(e.config.Risk.MaxSeverity)]

	if reqSev > maxSev {
		result.Violations = append(result.Violations, Violation{
			Rule: "RISK_SEVERITY", Value: req.Severity, Message: fmt.Sprintf("Action severity %s exceeds max %s", req.Severity, e.config.Risk.MaxSeverity),
		})
	}

	if req.CVSS > e.config.Risk.MaxCVSS {
		result.Violations = append(result.Violations, Violation{
			Rule: "RISK_CVSS", Value: fmt.Sprintf("%.1f", req.CVSS), Message: fmt.Sprintf("CVSS %.1f exceeds max %.1f", req.CVSS, e.config.Risk.MaxCVSS),
		})
	}

	if req.RiskLevel > e.config.Risk.MaxExploitRisk {
		result.Violations = append(result.Violations, Violation{
			Rule: "RISK_LEVEL", Value: fmt.Sprintf("%d", req.RiskLevel), Message: "Risk level exceeds maximum allowed",
		})
	}

	if req.IsDestructive && !e.config.Risk.AllowDestructive {
		result.Violations = append(result.Violations, Violation{
			Rule: "RISK_DESTRUCTIVE", Value: "true", Message: "Destructive operations not allowed",
		})
	}
	if req.IsPersistence && !e.config.Risk.AllowPersistence {
		result.Violations = append(result.Violations, Violation{
			Rule: "RISK_PERSISTENCE", Value: "true", Message: "Persistence operations not allowed",
		})
	}
	if req.IsExfiltration && !e.config.Risk.AllowExfiltration {
		result.Violations = append(result.Violations, Violation{
			Rule: "RISK_EXFIL", Value: "true", Message: "Exfiltration operations not allowed",
		})
	}
}

func (e *EnforcementEngine) checkAutoApprove(req *ActionRequest, result *EnforcementResult) {
	if containsString(e.config.Approval.AutoApproveTargets, req.TargetIP) {
		result.ApprovedActions = append(result.ApprovedActions, "auto_approve_target")
	}
	if containsInt(e.config.Approval.AutoApprovePorts, req.TargetPort) {
		result.ApprovedActions = append(result.ApprovedActions, "auto_approve_port")
	}
	if containsString(e.config.Approval.AutoApproveActions, req.Action) {
		result.ApprovedActions = append(result.ApprovedActions, fmt.Sprintf("auto_approve:%s", req.Action))
	}
	if req.RiskLevel <= e.config.Approval.MaxAutoApproveRisk && len(result.ApprovedActions) > 0 {
		result.RequiresApproval = false
	}
}

func (e *EnforcementEngine) checkApproval(req *ActionRequest, result *EnforcementResult) {
	if e.config.Approval.RequireApproval {
		result.RequiresApproval = true
	}
	if req.Confidence < e.config.Approval.MinConfidenceThreshold {
		result.RequiresApproval = true
		if result.Decision == DecisionAllowed {
			result.Decision = DecisionEscalated
		}
		result.Confidence = req.Confidence
	}

	if result.Allowed && result.RequiresApproval {
		result.Decision = DecisionEscalated
		if len(result.ApprovedActions) == 0 {
			result.Reason = "Action requires manual approval"
		}
	}
}

func (e *EnforcementEngine) CalculateAIConfidence(ctx interface{}, planConfidence, evidenceQuality float64, hasEvidence bool) float64 {
	cfg := e.config.AI
	if planConfidence < cfg.MinPlanConfidence {
		return planConfidence * 0.5
	}
	score := planConfidence * 0.6
	if hasEvidence {
		score += evidenceQuality * 0.3
	}
	if score >= cfg.MinPlanConfidence {
		return score
	}
	return score
}

func (e *EnforcementEngine) MustAbort(req *ActionRequest) (bool, string) {
	result := e.Evaluate(req)
	if !result.Allowed {
		for _, v := range result.Violations {
			if v.Rule == "DENY_NETWORK" || v.Rule == "RISK_DESTRUCTIVE" || v.Rule == "RISK_EXFIL" {
				return true, fmt.Sprintf("critical violation: %s", v.Message)
			}
		}
	}
	return false, ""
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func containsInt(slice []int, item int) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}
