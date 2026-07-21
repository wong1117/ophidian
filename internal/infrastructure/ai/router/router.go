package router

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
)

type RouteStrategy int

const (
	StrategyLocalFirst RouteStrategy = iota
	StrategyCloudFirst
	StrategyHeavyLocal
	StrategyFastCloud
	StrategyRoundRobin
	StrategyLocalOnly
	StrategyCloudOnly
)

type AIRouter struct {
	mu        sync.RWMutex
	local     ai.Provider
	cloud     ai.Provider
	fallback  ai.Provider
	strategy  RouteStrategy
	tracker   *TokenTracker
	metrics   *RouterMetrics
}

type RouterConfig struct {
	LocalEndpoint  string
	CloudProvider  ai.Provider
	LocalModel     string
	CloudModel     string
	Strategy       RouteStrategy
	HeavyThreshold int
}

type TokenTracker struct {
	mu      sync.Mutex
	entries []TokenEntry
}

type TokenEntry struct {
	Provider      string
	Model         string
	PromptTokens  int
	CompTokens    int
	TotalTokens   int
	Latency       time.Duration
	Timestamp     time.Time
	Phase         string
}

type RouterMetrics struct {
	LocalRequests  int64
	CloudRequests  int64
	LocalTokens    int64
	CloudTokens    int64
	LocalLatency   time.Duration
	CloudLatency   time.Duration
	mu             sync.Mutex
}

func NewAIRouter(cfg RouterConfig) *AIRouter {
	localProvider := newLocalAIProvider(cfg.LocalEndpoint, cfg.LocalModel)
	return &AIRouter{
		local:    localProvider,
		cloud:    cfg.CloudProvider,
		fallback: localProvider,
		strategy: cfg.Strategy,
		tracker:  &TokenTracker{},
		metrics:  &RouterMetrics{},
	}
}

func (r *AIRouter) SetStrategy(s RouteStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = s
}

func (r *AIRouter) Generate(ctx context.Context, req ai.GenerateRequest) (*ai.GenerateResponse, error) {
	r.mu.RLock()
	strategy := r.strategy
	r.mu.RUnlock()

	var provider ai.Provider
	switch strategy {
	case StrategyLocalFirst:
		provider = r.selectOrFallback(r.local, r.cloud)
	case StrategyCloudFirst:
		provider = r.selectOrFallback(r.cloud, r.local)
	case StrategyHeavyLocal:
		if req.MaxTokens > 500 {
			provider = r.local
		} else {
			provider = r.selectOrFallback(r.cloud, r.local)
		}
	case StrategyFastCloud:
		if req.MaxTokens <= 500 {
			provider = r.cloud
		} else {
			provider = r.selectOrFallback(r.local, r.cloud)
		}
	case StrategyLocalOnly:
		provider = r.local
	case StrategyCloudOnly:
		if r.cloud != nil {
			provider = r.cloud
		} else {
			provider = r.local
		}
	default:
		provider = r.selectOrFallback(r.local, r.cloud)
	}

	start := time.Now()
	resp, err := provider.Generate(ctx, req)
	latency := time.Since(start)

	if provider == r.local {
		r.metrics.mu.Lock()
		r.metrics.LocalRequests++
		r.metrics.LocalLatency += latency
		r.metrics.mu.Unlock()
	} else {
		r.metrics.mu.Lock()
		r.metrics.CloudRequests++
		r.metrics.CloudLatency += latency
		r.metrics.mu.Unlock()
	}

	if resp != nil {
		r.metrics.mu.Lock()
		if provider == r.local {
			r.metrics.LocalTokens += int64(resp.TokenUsage.TotalTokens)
		} else {
			r.metrics.CloudTokens += int64(resp.TokenUsage.TotalTokens)
		}
		r.metrics.mu.Unlock()

		r.tracker.mu.Lock()
		r.tracker.entries = append(r.tracker.entries, TokenEntry{
			Provider:     resp.Provider,
			Model:        resp.Model,
			PromptTokens: resp.TokenUsage.PromptTokens,
			CompTokens:   resp.TokenUsage.CompletionTokens,
			TotalTokens:  resp.TokenUsage.TotalTokens,
			Latency:      latency,
			Timestamp:    time.Now(),
		})
		r.tracker.mu.Unlock()
	}

	return resp, err
}

func (r *AIRouter) selectOrFallback(primary, secondary ai.Provider) ai.Provider {
	if primary != nil && primary.IsAvailable() {
		return primary
	}
	if secondary != nil && secondary.IsAvailable() {
		return secondary
	}
	return r.fallback
}

func (r *AIRouter) GetTokenUsage() TokenUsageSummary {
	r.tracker.mu.Lock()
	defer r.tracker.mu.Unlock()

	var summary TokenUsageSummary
	now := time.Now()
	for _, e := range r.tracker.entries {
		summary.TotalTokens += int64(e.TotalTokens)
		if e.Timestamp.After(now.Add(-24 * time.Hour)) {
			summary.Tokens24h += int64(e.TotalTokens)
		}
		if e.Timestamp.After(now.Add(-1 * time.Hour)) {
			summary.Tokens1h += int64(e.TotalTokens)
		}
		summary.TotalRequests++
	}

	r.metrics.mu.Lock()
	summary.LocalRequests = r.metrics.LocalRequests
	summary.CloudRequests = r.metrics.CloudRequests
	summary.LocalTokens = r.metrics.LocalTokens
	summary.CloudTokens = r.metrics.CloudTokens
	r.metrics.mu.Unlock()

	return summary
}

type TokenUsageSummary struct {
	TotalTokens    int64
	Tokens24h      int64
	Tokens1h       int64
	TotalRequests  int64
	LocalRequests  int64
	CloudRequests  int64
	LocalTokens    int64
	CloudTokens    int64
}

func (r *AIRouter) PrometheusMetrics() string {
	var sb strings.Builder
	s := r.GetTokenUsage()

	sb.WriteString("# HELP ai_requests_total Total AI requests\n")
	sb.WriteString("# TYPE ai_requests_total counter\n")
	sb.WriteString(fmt.Sprintf("ai_requests_total{provider=\"local\"} %d\n", s.LocalRequests))
	sb.WriteString(fmt.Sprintf("ai_requests_total{provider=\"cloud\"} %d\n", s.CloudRequests))

	sb.WriteString("# HELP ai_tokens_total Total tokens used\n")
	sb.WriteString("# TYPE ai_tokens_total counter\n")
	sb.WriteString(fmt.Sprintf("ai_tokens_total{provider=\"local\"} %d\n", s.LocalTokens))
	sb.WriteString(fmt.Sprintf("ai_tokens_total{provider=\"cloud\"} %d\n", s.CloudTokens))
	sb.WriteString(fmt.Sprintf("ai_tokens_total{period=\"1h\"} %d\n", s.Tokens1h))
	sb.WriteString(fmt.Sprintf("ai_tokens_total{period=\"24h\"} %d\n", s.Tokens24h))
	sb.WriteString(fmt.Sprintf("ai_tokens_total{period=\"total\"} %d\n", s.TotalTokens))

	return sb.String()
}

type localAIProvider struct {
	endpoint  string
	model     string
	available atomic.Bool
}

func newLocalAIProvider(endpoint, model string) *localAIProvider {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3"
	}
	p := &localAIProvider{endpoint: endpoint, model: model}
	p.available.Store(true)
	return p
}

func (p *localAIProvider) Name() string { return "ollama" }

func (p *localAIProvider) Generate(ctx context.Context, req ai.GenerateRequest) (*ai.GenerateResponse, error) {
	if !p.available.Load() {
		return nil, fmt.Errorf("local AI provider is not available")
	}
	return &ai.GenerateResponse{
		Content:  fmt.Sprintf("[local:%s] response to: %s", p.model, truncate(req.Prompt, 50)),
		Model:    p.model,
		Provider: "ollama",
		TokenUsage: ai.TokenUsage{
			PromptTokens:     estimateTokens(req.Prompt),
			CompletionTokens: estimateTokens(req.Prompt) / 2,
			TotalTokens:      estimateTokens(req.Prompt) * 3 / 2,
		},
		FinishReason: "stop",
	}, nil
}

func (p *localAIProvider) GenerateStream(ctx context.Context, req ai.GenerateRequest) (<-chan string, error) {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		resp, err := p.Generate(ctx, req)
		if err == nil {
			ch <- resp.Content
		}
	}()
	return ch, nil
}

func (p *localAIProvider) IsAvailable() bool  { return p.available.Load() }
func (p *localAIProvider) SetAvailable(v bool)  { p.available.Store(v) }

func estimateTokens(text string) int {
	words := len(strings.Fields(text))
	chars := len(text)
	return (words + chars/4) / 2
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "..."
}

type PromptTemplate struct {
	Name    string
	System  string
	User    string
	Phase   string
}

type PromptLibrary struct {
	mu        sync.RWMutex
	templates map[string]PromptTemplate
}

func NewPromptLibrary() *PromptLibrary {
	lib := &PromptLibrary{templates: make(map[string]PromptTemplate)}
	lib.loadDefaults()
	return lib
}

func (p *PromptLibrary) loadDefaults() {
	p.templates["recon-passive"] = PromptTemplate{
		Name:   "Passive Reconnaissance",
		System: "You are an expert offensive security reconnaissance analyst. Your job is to analyze target information and identify reconnaissance opportunities.",
		User:   "Analyze the following target: {{target}}. Identify potential passive reconnaissance techniques including DNS enumeration, WHOIS lookup, SSL certificate analysis, and subdomain discovery. Focus on: {{focus}}",
		Phase:  "RECON",
	}
	p.templates["recon-active"] = PromptTemplate{
		Name:   "Active Reconnaissance",
		System: "You are an expert penetration tester specializing in active reconnaissance. Analyze scan results and recommend next steps.",
		User:   "Target: {{target}}\nOpen ports: {{ports}}\nServices detected: {{services}}\nGenerate a prioritized list of active reconnaissance actions.",
		Phase:  "RECON",
	}
	p.templates["exploit-plan"] = PromptTemplate{
		Name:   "Exploit Planning",
		System: "You are an elite exploit developer. Given target information, vulnerability data, and RoE constraints, generate an optimal attack plan.",
		User:   "Mission: {{mission}}\nTarget: {{target}}\nVulnerabilities: {{vulns}}\nRoE Constraints: {{roe}}\nGenerate a step-by-step exploitation plan with confidence scores.",
		Phase:  "EXPLOIT",
	}
	p.templates["exploit-execute"] = PromptTemplate{
		Name:   "Exploit Execution",
		System: "You are an automated exploitation engine. Execute the given exploit and report results.",
		User:   "Execute exploit {{exploit_id}} against target {{target}} on ports {{ports}}. Payload options: {{options}}. Report success/failure and post-exploitation opportunities.",
		Phase:  "EXPLOIT",
	}
	p.templates["report-summary"] = PromptTemplate{
		Name:   "Report Summary",
		System: "You are a senior security consultant writing an executive report. Summarize findings clearly for both technical and non-technical audiences.",
		User:   "Findings: {{findings}}\nGenerate an executive summary with risk ratings, impact assessment, and prioritized remediation recommendations.",
		Phase:  "REPORT",
	}
	p.templates["report-technical"] = PromptTemplate{
		Name:   "Technical Report",
		System: "You are a security researcher writing a detailed technical report. Include reproduction steps, evidence, and technical analysis.",
		User:   "Mission: {{mission}}\nFindings: {{findings}}\nEvidence: {{evidence}}\nGenerate a detailed technical report with CVSS scores, CVE references, and remediation guidance.",
		Phase:  "REPORT",
	}
}

func (p *PromptLibrary) Render(name string, vars map[string]string) (string, string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tmpl, ok := p.templates[name]
	if !ok {
		return "", "", fmt.Errorf("prompt template not found: %s", name)
	}

	system := tmpl.System
	user := tmpl.User
	for k, v := range vars {
		system = strings.ReplaceAll(system, "{{"+k+"}}", v)
		user = strings.ReplaceAll(user, "{{"+k+"}}", v)
	}

	return system, user, nil
}

func (p *PromptLibrary) GetByPhase(phase string) []PromptTemplate {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []PromptTemplate
	for _, t := range p.templates {
		if strings.EqualFold(t.Phase, phase) {
			result = append(result, t)
		}
	}
	return result
}
