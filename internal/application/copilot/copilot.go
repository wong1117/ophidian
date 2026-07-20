package copilot

import (
	"context"
	"fmt"
	"strings"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type IntentType string

const (
	IntentRecon       IntentType = "RECON"
	IntentExploit     IntentType = "EXPLOIT"
	IntentPrivEsc     IntentType = "PRIVILEGE_ESCALATION"
	IntentLateral     IntentType = "LATERAL_MOVEMENT"
	IntentPersistence IntentType = "PERSISTENCE"
	IntentExfil       IntentType = "EXFILTRATION"
	IntentCleanup     IntentType = "CLEANUP"
	IntentReport      IntentType = "REPORT"
)

type WorkflowStep struct {
	Order       int
	Intent      IntentType
	Description string
	Technique   string
	Tool        string
	Parameters  map[string]interface{}
	DependsOn   []int
	RequiresApproval bool
}

type CopilotChain struct {
	NaturalLanguage string
	ParsedIntents   []ParsedIntent
	Workflow        []WorkflowStep
	MissionID       string
	TargetID        string
	RequiresApproval bool
}

type ParsedIntent struct {
	Type        IntentType
	Target      string
	Confidence  float64
	Constraints map[string]string
}

type Parser interface {
	Parse(ctx context.Context, nlp string) ([]ParsedIntent, error)
}

type LLM interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type ChainBuilder struct {
	parser    Parser
	llm       LLM
	templates map[IntentType][]WorkflowStep
}

func NewChainBuilder(parser Parser, llm LLM) *ChainBuilder {
	cb := &ChainBuilder{
		parser:    parser,
		llm:       llm,
		templates: make(map[IntentType][]WorkflowStep),
	}
	cb.initTemplates()
	return cb
}

func (cb *ChainBuilder) initTemplates() {
	cb.templates[IntentRecon] = []WorkflowStep{
		{Order: 1, Intent: IntentRecon, Technique: "passive_recon", Description: "Passive reconnaissance (OSINT, DNS)", RequiresApproval: false},
		{Order: 2, Intent: IntentRecon, Technique: "active_recon", Description: "Active reconnaissance (port scan, service detection)", RequiresApproval: false},
		{Order: 3, Intent: IntentRecon, Technique: "vuln_scan", Description: "Vulnerability scanning", RequiresApproval: true},
	}

	cb.templates[IntentExploit] = []WorkflowStep{
		{Order: 1, Intent: IntentExploit, Technique: "identify_weakness", Description: "Identify weakness from recon data", RequiresApproval: false},
		{Order: 2, Intent: IntentExploit, Technique: "prepare_exploit", Description: "Prepare exploit payload", RequiresApproval: true},
		{Order: 3, Intent: IntentExploit, Technique: "execute_exploit", Description: "Execute exploit against target", RequiresApproval: true},
		{Order: 4, Intent: IntentExploit, Technique: "verify_access", Description: "Verify access level achieved", RequiresApproval: false},
	}

	cb.templates[IntentPrivEsc] = []WorkflowStep{
		{Order: 1, Intent: IntentPrivEsc, Technique: "enum_system", Description: "Enumerate system for privilege escalation vectors", RequiresApproval: false},
		{Order: 2, Intent: IntentPrivEsc, Technique: "exploit_privsec", Description: "Execute privilege escalation", RequiresApproval: true},
		{Order: 3, Intent: IntentPrivEsc, Technique: "verify_priv", Description: "Verify new privilege level", RequiresApproval: false},
	}

	cb.templates[IntentLateral] = []WorkflowStep{
		{Order: 1, Intent: IntentLateral, Technique: "discover_peers", Description: "Discover peer hosts on network", RequiresApproval: false},
		{Order: 2, Intent: IntentLateral, Technique: "enum_targets", Description: "Enumerate lateral movement targets", RequiresApproval: false},
		{Order: 3, Intent: IntentLateral, Technique: "move_lateral", Description: "Execute lateral movement", RequiresApproval: true},
	}

	cb.templates[IntentPersistence] = []WorkflowStep{
		{Order: 1, Intent: IntentPersistence, Technique: "enum_persistence", Description: "Enumerate existing persistence mechanisms", RequiresApproval: false},
		{Order: 2, Intent: IntentPersistence, Technique: "install_persistence", Description: "Install persistence mechanism", RequiresApproval: true},
	}

	cb.templates[IntentExfil] = []WorkflowStep{
		{Order: 1, Intent: IntentExfil, Technique: "find_target_data", Description: "Find target data for exfiltration", RequiresApproval: false},
		{Order: 2, Intent: IntentExfil, Technique: "prepare_channel", Description: "Prepare covert exfiltration channel", RequiresApproval: true},
		{Order: 3, Intent: IntentExfil, Technique: "exfiltrate", Description: "Execute data exfiltration", RequiresApproval: true},
	}

	cb.templates[IntentCleanup] = []WorkflowStep{
		{Order: 1, Intent: IntentCleanup, Technique: "enum_artifacts", Description: "Enumerate artifacts left behind", RequiresApproval: false},
		{Order: 2, Intent: IntentCleanup, Technique: "clean_artifacts", Description: "Remove all artifacts", RequiresApproval: true},
	}

	cb.templates[IntentReport] = []WorkflowStep{
		{Order: 1, Intent: IntentReport, Technique: "collect_evidence", Description: "Collect all evidence", RequiresApproval: false},
		{Order: 2, Intent: IntentReport, Technique: "generate_report", Description: "Generate engagement report", RequiresApproval: false},
	}
}

func (cb *ChainBuilder) Build(ctx context.Context, missionID, targetID, nlp string) (*CopilotChain, error) {
	intents, err := cb.parser.Parse(ctx, nlp)
	if err != nil {
		return nil, err
	}

	chain := &CopilotChain{
		NaturalLanguage: nlp,
		ParsedIntents:   intents,
		MissionID:       missionID,
		TargetID:        targetID,
	}

	stepOrder := 0
	for _, intent := range intents {
		template, ok := cb.templates[intent.Type]
		if !ok {
			continue
		}
		for _, step := range template {
			stepOrder++
			step.Order = stepOrder
			chain.Workflow = append(chain.Workflow, step)
		}
	}

	for _, step := range chain.Workflow {
		if step.RequiresApproval {
			chain.RequiresApproval = true
			break
		}
	}

	return chain, nil
}

type SimpleParser struct{}

func NewSimpleParser() *SimpleParser {
	return &SimpleParser{}
}

func (p *SimpleParser) Parse(ctx context.Context, nlp string) ([]ParsedIntent, error) {
	nlp = strings.ToLower(nlp)
	var intents []ParsedIntent

	intentMap := map[string]IntentType{
		"domain admin":       IntentPrivEsc,
		"admin access":        IntentPrivEsc,
		"privilege escalation": IntentPrivEsc,
		"recon":              IntentRecon,
		"reconnaissance":     IntentRecon,
		"scan":               IntentRecon,
		"exploit":            IntentExploit,
		"hack":               IntentExploit,
		"compromise":         IntentExploit,
		"lateral":            IntentLateral,
		"move":               IntentLateral,
		"pivot":              IntentLateral,
		"persist":            IntentPersistence,
		"persistence":        IntentPersistence,
		"backdoor":           IntentPersistence,
		"exfil":              IntentExfil,
		"exfiltrate":         IntentExfil,
		"steal":              IntentExfil,
		"clean":              IntentCleanup,
		"cleanup":            IntentCleanup,
		"report":             IntentReport,
	}

	for keyword, intentType := range intentMap {
		if strings.Contains(nlp, keyword) {
			confidence := 0.7
			if strings.HasPrefix(nlp, keyword) || strings.Contains(nlp, keyword+" ") {
				confidence = 0.9
			}
			intents = append(intents, ParsedIntent{
				Type:       intentType,
				Confidence: confidence,
			})
		}
	}

	if len(intents) == 0 {
		intents = append(intents, ParsedIntent{
			Type:       IntentRecon,
			Confidence: 0.5,
		})
	}

	return intents, nil
}
