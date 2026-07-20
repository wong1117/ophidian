package router

import (
	"context"
	"strings"
	"testing"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
	"github.com/stretchr/testify/assert"
)

type mockCloudProvider struct {
	available bool
}

func (m *mockCloudProvider) Name() string { return "openai" }
func (m *mockCloudProvider) Generate(ctx context.Context, req ai.GenerateRequest) (*ai.GenerateResponse, error) {
	return &ai.GenerateResponse{
		Content: "cloud-response",
		Model:   "gpt-4",
		Provider: "openai",
		TokenUsage: ai.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
		FinishReason: "stop",
	}, nil
}
func (m *mockCloudProvider) GenerateStream(ctx context.Context, req ai.GenerateRequest) (<-chan string, error) { return nil, nil }
func (m *mockCloudProvider) IsAvailable() bool { return m.available }

func TestAIRouter_Generate_LocalFirst(t *testing.T) {
	cloud := &mockCloudProvider{available: true}
	router := NewAIRouter(RouterConfig{
		LocalEndpoint: "",
		CloudProvider: cloud,
		CloudModel:    "gpt-4",
		Strategy:      StrategyLocalFirst,
	})

	resp, err := router.Generate(context.Background(), ai.GenerateRequest{Prompt: "hello", MaxTokens: 100})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Content, "local")
	assert.Equal(t, "ollama", resp.Provider)
}

func TestAIRouter_Generate_CloudOnly(t *testing.T) {
	cloud := &mockCloudProvider{available: true}
	router := NewAIRouter(RouterConfig{
		LocalEndpoint: "",
		CloudProvider: cloud,
		CloudModel:    "gpt-4",
		Strategy:      StrategyCloudOnly,
	})

	resp, err := router.Generate(context.Background(), ai.GenerateRequest{Prompt: "hello"})

	assert.NoError(t, err)
	assert.Equal(t, "cloud-response", resp.Content)
	assert.Equal(t, "openai", resp.Provider)
}

func TestAIRouter_Generate_HeavyLocal(t *testing.T) {
	cloud := &mockCloudProvider{available: true}
	router := NewAIRouter(RouterConfig{
		LocalEndpoint: "",
		CloudProvider: cloud,
		CloudModel:    "gpt-4",
		Strategy:      StrategyHeavyLocal,
	})

	resp, err := router.Generate(context.Background(), ai.GenerateRequest{Prompt: "complex reasoning task", MaxTokens: 1000})

	assert.NoError(t, err)
	assert.Contains(t, resp.Content, "local")
}

func TestAIRouter_Generate_FastCloud(t *testing.T) {
	cloud := &mockCloudProvider{available: true}
	router := NewAIRouter(RouterConfig{
		LocalEndpoint: "",
		CloudProvider: cloud,
		CloudModel:    "gpt-4",
		Strategy:      StrategyFastCloud,
	})

	resp, err := router.Generate(context.Background(), ai.GenerateRequest{Prompt: "simple query", MaxTokens: 100})

	assert.NoError(t, err)
	assert.Equal(t, "cloud-response", resp.Content)
}

func TestAIRouter_GetTokenUsage(t *testing.T) {
	cloud := &mockCloudProvider{available: true}
	router := NewAIRouter(RouterConfig{
		LocalEndpoint: "",
		CloudProvider: cloud,
		CloudModel:    "gpt-4",
		Strategy:      StrategyLocalFirst,
	})

	router.Generate(context.Background(), ai.GenerateRequest{Prompt: "test", MaxTokens: 50})
	router.Generate(context.Background(), ai.GenerateRequest{Prompt: "test", MaxTokens: 50})

	summary := router.GetTokenUsage()
	assert.Greater(t, summary.TotalRequests, int64(0))
	assert.Greater(t, summary.TotalTokens, int64(0))
}

func TestAIRouter_PrometheusMetrics(t *testing.T) {
	cloud := &mockCloudProvider{available: true}
	router := NewAIRouter(RouterConfig{
		LocalEndpoint: "",
		CloudProvider: cloud,
		CloudModel:    "gpt-4",
		Strategy:      StrategyLocalFirst,
	})

	router.Generate(context.Background(), ai.GenerateRequest{Prompt: "test"})

	metrics := router.PrometheusMetrics()
	assert.Contains(t, metrics, "ai_requests_total")
	assert.Contains(t, metrics, "ai_tokens_total")
	assert.Contains(t, metrics, "provider=\"local\"")
}

func TestPromptLibrary_Render(t *testing.T) {
	lib := NewPromptLibrary()

	system, user, err := lib.Render("recon-passive", map[string]string{
		"target": "example.com",
		"focus":  "subdomain enumeration",
	})

	assert.NoError(t, err)
	assert.Contains(t, system, "offensive security reconnaissance")
	assert.Contains(t, user, "example.com")
	assert.Contains(t, user, "subdomain enumeration")
}

func TestPromptLibrary_Render_Missing(t *testing.T) {
	lib := NewPromptLibrary()
	_, _, err := lib.Render("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPromptLibrary_GetByPhase(t *testing.T) {
	lib := NewPromptLibrary()

	recon := lib.GetByPhase("RECON")
	assert.Greater(t, len(recon), 0)

	exploit := lib.GetByPhase("EXPLOIT")
	assert.Greater(t, len(exploit), 1)

	report := lib.GetByPhase("REPORT")
	assert.Greater(t, len(report), 1)
}

func TestEstimateTokens(t *testing.T) {
	tokens := estimateTokens("hello world this is a test")
	assert.Greater(t, tokens, 5)
	assert.Less(t, tokens, 50)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello...", truncate("hello world", 5))
	assert.Equal(t, "short", truncate("short", 10))
}

func TestRouter_StrategySwitch(t *testing.T) {
	cloud := &mockCloudProvider{available: true}
	router := NewAIRouter(RouterConfig{
		LocalEndpoint: "",
		CloudProvider: cloud,
		Strategy:      StrategyLocalFirst,
	})

	resp1, _ := router.Generate(context.Background(), ai.GenerateRequest{Prompt: "test"})
	assert.Contains(t, resp1.Content, "local")

	router.SetStrategy(StrategyCloudOnly)
	resp2, _ := router.Generate(context.Background(), ai.GenerateRequest{Prompt: "test"})
	assert.Equal(t, "cloud-response", resp2.Content)
}

func TestPromptLibrary_AllPhases(t *testing.T) {
	lib := NewPromptLibrary()

	for _, name := range []string{"recon-passive", "recon-active", "exploit-plan", "exploit-execute", "report-summary", "report-technical"} {
		system, user, err := lib.Render(name, map[string]string{
			"target": "test-target",
			"mission": "test-mission",
			"focus": "test-focus",
			"vulns": "CVE-2024-0001",
			"ports": "80,443",
			"services": "http,https",
			"findings": "SQL Injection, XSS",
			"evidence": "log data",
			"exploit_id": "exp-01",
			"options": "LHOST=10.0.0.1",
			"roe": "No destructive ops",
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, system)
		assert.NotEmpty(t, user)
		assert.False(t, strings.Contains(user, "{{"))
	}
}
