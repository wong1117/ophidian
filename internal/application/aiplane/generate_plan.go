package aiplane

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type VectorStore interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

type SearchResult struct {
	ID       string
	Content  string
	Score    float64
	Metadata map[string]interface{}
}

type GeneratePlanUseCase struct {
	planner     attackplan.AIPlanner
	planRepo    attackplan.AttackPlanRepository
	missionRepo mission.MissionRepository
	llmClient   LLMClient
	vectorStore VectorStore
	eventStore  EventStore
}

func NewGeneratePlanUseCase(
	planner attackplan.AIPlanner,
	planRepo attackplan.AttackPlanRepository,
	missionRepo mission.MissionRepository,
	llmClient LLMClient,
	vectorStore VectorStore,
	eventStore EventStore,
) *GeneratePlanUseCase {
	return &GeneratePlanUseCase{
		planner:     planner,
		planRepo:    planRepo,
		missionRepo: missionRepo,
		llmClient:   llmClient,
		vectorStore: vectorStore,
		eventStore:  eventStore,
	}
}

type ExecuteRequest struct {
	MissionID string
}

type ExecuteResponse struct {
	Plan *dto.AttackPlanGeneratedResponse
}

func (uc *GeneratePlanUseCase) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	if req.MissionID == "" {
		return nil, fmt.Errorf("%w: mission id is required", common.ErrInvalidID)
	}

	m, err := uc.missionRepo.FindByID(ctx, req.MissionID)
	if err != nil {
		return nil, fmt.Errorf("fetch mission: %w", err)
	}

	if err := validateMissionForPlanning(m); err != nil {
		return nil, err
	}

	if err := validateMissionTarget(m); err != nil {
		return nil, err
	}

	profile := buildTargetProfile(m)
	constraints := mapRoE(m.RoE)

	vectorContext, err := uc.retrieveTTPContext(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("retrieve ttp context: %w", err)
	}

	llmPrompt := buildPlanPrompt(profile, constraints, vectorContext)
	llmRaw, err := uc.llmClient.Generate(ctx, llmPrompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	parsed, err := parseLLMOutput(llmRaw)
	if err != nil {
		return nil, fmt.Errorf("parse llm output: %w", err)
	}

	rankedPaths, err := uc.planner.RankPaths(ctx, parsed.Graph)
	if err != nil {
		return nil, fmt.Errorf("rank paths: %w", err)
	}

	ctxID := common.NewID()
	plan := &attackplan.AttackPlan{
		ID:          ctxID,
		MissionID:   m.ID,
		Graph:       parsed.Graph,
		RankedPaths: rankedPaths,
		Rationale:   parsed.Rationale,
		Status:      attackplan.PlanDraft,
		CreatedAt:   common.Now(),
		UpdatedAt:   common.Now(),
	}

	confidence, err := uc.planner.EvaluateConfidence(ctx, plan, evidenceFromProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("evaluate confidence: %w", err)
	}
	plan.Confidence = confidence

	if err := uc.planRepo.Save(ctx, plan); err != nil {
		return nil, fmt.Errorf("save plan: %w", err)
	}

	event := attackplan.PlanGenerated{
		PlanID:    ctxID,
		MissionID: m.ID,
		Graph:     parsed.Graph,
		Timestamp: common.Now(),
	}
	if err := uc.eventStore.Append(ctx, event); err != nil {
		return nil, fmt.Errorf("append event: %w", err)
	}

	return &ExecuteResponse{Plan: mapToPlanResponseDTO(plan)}, nil
}

func validateMissionForPlanning(m *mission.Mission) error {
	if m.Status != mission.MissionDraft && m.Status != mission.MissionActive {
		return fmt.Errorf("%w: mission status %s is not eligible for planning", common.ErrInvalidState, m.Status)
	}
	if len(m.Objectives) == 0 {
		return fmt.Errorf("%w: mission has no objectives defined", common.ErrInvalidState)
	}
	return nil
}

func validateMissionTarget(m *mission.Mission) error {
	if len(m.Target.IPs) == 0 && len(m.Target.Domains) == 0 && len(m.Target.CIDRs) == 0 {
		return fmt.Errorf("%w: mission target has no ips, domains, or cidrs defined", common.ErrInvalidTarget)
	}
	if m.Target.Name == "" {
		return fmt.Errorf("%w: mission target name is required", common.ErrInvalidTarget)
	}
	return nil
}

func buildTargetProfile(m *mission.Mission) attackplan.TargetProfile {
	return attackplan.TargetProfile{
		IPs:       m.Target.IPs,
		Domains:   m.Target.Domains,
		Tags:      []string{m.Target.Name},
	}
}

func mapRoE(roe mission.RoEConstraints) attackplan.RoEConstraints {
	return attackplan.RoEConstraints{
		MaxSeverity:      string(roe.MaxSeverity),
		AllowDestructive: roe.AllowDestructive,
		AllowPersistence: roe.AllowPersistence,
		AllowExfiltration: roe.AllowExfiltration,
	}
}

func evidenceFromProfile(profile attackplan.TargetProfile) []attackplan.Evidence {
	var evidence []attackplan.Evidence
	if len(profile.IPs) > 0 {
		evidence = append(evidence, attackplan.Evidence{
			Type:    "TARGET_IPS",
			Content: strings.Join(profile.IPs, ","),
			Source:  "recon",
		})
	}
	if len(profile.Domains) > 0 {
		evidence = append(evidence, attackplan.Evidence{
			Type:    "TARGET_DOMAINS",
			Content: strings.Join(profile.Domains, ","),
			Source:  "recon",
		})
	}
	for _, svc := range profile.Services {
		evidence = append(evidence, attackplan.Evidence{
			ID:      fmt.Sprintf("svc-%s-%d", svc.Name, svc.Port),
			Type:    "SERVICE",
			Content: fmt.Sprintf("%s:%d %s %s", svc.Name, svc.Port, svc.Protocol, svc.Version),
			Source:  "recon",
		})
	}
	return evidence
}

func buildPastAttempts(tasks []mission.Task) []attackplan.PastAttempt {
	attempts := make([]attackplan.PastAttempt, 0)
	for _, t := range tasks {
		if t.Status == common.TaskFailed || t.Status == common.TaskSuccess {
			result := "success"
			if t.Status == common.TaskFailed {
				result = "failed"
			}
			errStr := ""
			if t.Result != nil && t.Result.Error != nil {
				errStr = t.Result.Error.Message
			}
			targetID := ""
			if len(t.Parameters) > 0 {
				if tid, ok := t.Parameters["target_id"].(string); ok {
					targetID = tid
				}
			}
			attempts = append(attempts, attackplan.PastAttempt{
				TaskID:    t.ID.String(),
				TargetID:  targetID,
				Timestamp: t.CreatedAt.Unix(),
				Result:    result,
				Error:     errStr,
			})
		}
	}
	return attempts
}

func (uc *GeneratePlanUseCase) retrieveTTPContext(ctx context.Context, profile attackplan.TargetProfile) (string, error) {
	var queries []string
	for _, ip := range profile.IPs {
		queries = append(queries, ip)
	}
	for _, domain := range profile.Domains {
		queries = append(queries, domain)
	}
	for _, svc := range profile.Services {
		if svc.Name != "" {
			queries = append(queries, svc.Name)
		}
		if svc.Version != "" {
			queries = append(queries, fmt.Sprintf("%s %s", svc.Name, svc.Version))
		}
	}
	for _, tag := range profile.Tags {
		queries = append(queries, tag)
	}
	if profile.OS != "" {
		queries = append(queries, profile.OS)
	}
	if len(queries) == 0 {
		return "", nil
	}

	query := strings.Join(queries, " ")
	results, err := uc.vectorStore.Search(ctx, query, 5)
	if err != nil {
		return "", fmt.Errorf("vector search: %w", err)
	}

	if len(results) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("Relevant TTP intelligence:\n")
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] %s (confidence: %.2f)\n", i+1, r.Content, r.Score))
	}
	return sb.String(), nil
}

func buildPlanPrompt(profile attackplan.TargetProfile, constraints attackplan.RoEConstraints, vectorContext string) string {
	var sb strings.Builder
	sb.WriteString("You are an offensive security expert. Generate an attack plan in JSON format based on the given target profile and constraints.\n\n")

	sb.WriteString("## Target Profile\n")
	if len(profile.IPs) > 0 {
		sb.WriteString(fmt.Sprintf("IPs: %s\n", strings.Join(profile.IPs, ", ")))
	}
	if len(profile.Domains) > 0 {
		sb.WriteString(fmt.Sprintf("Domains: %s\n", strings.Join(profile.Domains, ", ")))
	}
	if profile.OS != "" {
		sb.WriteString(fmt.Sprintf("OS: %s\n", profile.OS))
	}
	for _, svc := range profile.Services {
		sb.WriteString(fmt.Sprintf("Service: %s:%d (%s %s)\n", svc.Name, svc.Port, svc.Protocol, svc.Version))
	}
	if len(profile.OpenPorts) > 0 {
		sb.WriteString(fmt.Sprintf("Open Ports: %v\n", profile.OpenPorts))
	}

	sb.WriteString("\n## Rules of Engagement\n")
	sb.WriteString(fmt.Sprintf("- Max Severity: %s\n", constraints.MaxSeverity))
	sb.WriteString(fmt.Sprintf("- Destructive Operations: %t\n", constraints.AllowDestructive))
	sb.WriteString(fmt.Sprintf("- Persistence: %t\n", constraints.AllowPersistence))
	sb.WriteString(fmt.Sprintf("- Exfiltration: %t\n", constraints.AllowExfiltration))

	if vectorContext != "" {
		sb.WriteString("\n## Threat Intelligence\n")
		sb.WriteString(vectorContext)
	}

	sb.WriteString("\n## Output Format\n")
	sb.WriteString("Return a valid JSON object with the following structure:\n")
	sb.WriteString(`{
  "graph": {
    "nodes": [
      {
        "id": "node_1",
        "target_id": "string",
        "type": "RECON|EXPLOIT|POST_EXPLOIT|PIVOT",
        "service": "string",
        "cve": "string",
        "confidence": 0.0-1.0,
        "risk_level": "LOW|MEDIUM|HIGH|CRITICAL"
      }
    ],
    "edges": [
      {
        "from": "node_1",
        "to": "node_2",
        "weight": 0.0-1.0,
        "confidence": 0.0-1.0,
        "condition": "string"
      }
    ]
  },
  "rationale": "Explanation of the attack strategy"
}
`)
	sb.WriteString("Only return the JSON object, no other text.\n")

	return sb.String()
}

type llmGraphOutput struct {
	Graph     llmGraph `json:"graph"`
	Rationale string   `json:"rationale"`
}

type llmGraph struct {
	Nodes []llmNode `json:"nodes"`
	Edges []llmEdge `json:"edges"`
}

type llmNode struct {
	ID         string  `json:"id"`
	TargetID   string  `json:"target_id"`
	Type       string  `json:"type"`
	Service    string  `json:"service"`
	CVE        string  `json:"cve"`
	Confidence float64 `json:"confidence"`
	RiskLevel  string  `json:"risk_level"`
}

type llmEdge struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Weight     float64 `json:"weight"`
	Confidence float64 `json:"confidence"`
	Condition  string  `json:"condition"`
}

type parsedPlan struct {
	Graph     attackplan.AttackGraph
	Rationale string
}

func parseLLMOutput(raw string) (*parsedPlan, error) {
	cleaned := extractJSON(raw)
	if cleaned == "" {
		return nil, fmt.Errorf("no json found in llm output")
	}

	var out llmGraphOutput
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return nil, fmt.Errorf("unmarshal llm output: %w", err)
	}

	nodes := make([]attackplan.Node, len(out.Graph.Nodes))
	for i, n := range out.Graph.Nodes {
		nodes[i] = attackplan.Node{
			ID:         n.ID,
			TargetID:   n.TargetID,
			Type:       attackplan.NodeType(n.Type),
			Service:    n.Service,
			CVE:        n.CVE,
			Confidence: n.Confidence,
			RiskLevel:  parseRiskLevel(n.RiskLevel),
		}
	}

	edges := make([]attackplan.Edge, len(out.Graph.Edges))
	for i, e := range out.Graph.Edges {
		edges[i] = attackplan.Edge{
			From:       e.From,
			To:         e.To,
			Weight:     e.Weight,
			Confidence: e.Confidence,
			Condition:  e.Condition,
		}
	}

	return &parsedPlan{
		Graph: attackplan.AttackGraph{
			Nodes: nodes,
			Edges: edges,
		},
		Rationale: out.Rationale,
	}, nil
}

func parseRiskLevel(s string) common.RiskLevel {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return common.RiskCritical
	case "HIGH":
		return common.RiskHigh
	case "MEDIUM":
		return common.RiskMedium
	default:
		return common.RiskLow
	}
}

func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}

func mapToPlanResponseDTO(plan *attackplan.AttackPlan) *dto.AttackPlanGeneratedResponse {
	paths := make([]dto.RankedPathDTO, len(plan.RankedPaths))
	for i, rp := range plan.RankedPaths {
		paths[i] = dto.RankedPathDTO{
			Steps:      rp.Nodes,
			Score:      rp.TotalScore,
			Confidence: rp.Confidence,
			RiskLevel:  string(rp.RiskLevel),
		}
	}

	nodes := make([]dto.NodeDTO, len(plan.Graph.Nodes))
	for i, n := range plan.Graph.Nodes {
		nodes[i] = dto.NodeDTO{
			ID:         n.ID,
			TargetID:   n.TargetID,
			Type:       string(n.Type),
			Service:    n.Service,
			CVE:        n.CVE,
			Confidence: n.Confidence,
			RiskLevel:  string(n.RiskLevel),
		}
	}

	edges := make([]dto.EdgeDTO, len(plan.Graph.Edges))
	for i, e := range plan.Graph.Edges {
		edges[i] = dto.EdgeDTO{
			From:       e.From,
			To:         e.To,
			Weight:     e.Weight,
			Confidence: e.Confidence,
			Condition:  e.Condition,
		}
	}

	return &dto.AttackPlanGeneratedResponse{
		PlanID:      plan.ID.String(),
		MissionID:   plan.MissionID.String(),
		Nodes:       nodes,
		Edges:       edges,
		RankedPaths: paths,
		Confidence:  plan.Confidence,
		Rationale:   plan.Rationale,
		ETA:         plan.ETA,
		Status:      string(plan.Status),
		CreatedAt:   plan.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
