package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
)

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Content []contentBlock `json:"content"`
	Usage   usageInfo      `json:"usage"`
}

type Provider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
	cfg     ai.ProviderConfig
}

func NewProvider(cfg ai.ProviderConfig) *Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120
	}
	return &Provider{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
		cfg:     cfg,
	}
}

func (p *Provider) Name() string { return string(ai.ProviderAnthropic) }

func (p *Provider) IsAvailable() bool {
	return p.apiKey != ""
}

func (p *Provider) Generate(ctx context.Context, req ai.GenerateRequest) (*ai.GenerateResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}

	body := map[string]interface{}{
		"model":      model,
		"max_tokens": req.MaxTokens,
		"messages": []map[string]interface{}{
			{"role": "user", "content": req.Prompt},
		},
	}

	if req.System != "" {
		body["system"] = req.System
	}

	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error: %s", resp.Status)
	}

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no response from Anthropic")
	}

	return &ai.GenerateResponse{
		Content:  result.Content[0].Text,
		Model:    result.Model,
		Provider: string(ai.ProviderAnthropic),
		TokenUsage: ai.TokenUsage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
		FinishReason: "stop",
	}, nil
}

func (p *Provider) GenerateStream(ctx context.Context, req ai.GenerateRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
