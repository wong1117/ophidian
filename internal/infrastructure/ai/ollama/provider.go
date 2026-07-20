package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
)

type Provider struct {
	baseURL string
	model   string
	client  *http.Client
	cfg     ai.ProviderConfig
}

type generateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system,omitempty"`
	Stream  bool           `json:"stream"`
	Options generateOptions `json:"options,omitempty"`
}

type generateOptions struct {
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	TopK        int     `json:"top_k"`
	MaxTokens   int     `json:"num_predict"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewProvider(cfg ai.ProviderConfig) *Provider {
	host := "localhost"
	port := 11434
	if cfg.BaseURL != "" {
		baseURL := cfg.BaseURL
		return &Provider{
			baseURL: baseURL,
			model:   cfg.Model,
			client:  &http.Client{Timeout: 120 * time.Second},
			cfg:     cfg,
		}
	}
	return &Provider{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		model:   cfg.Model,
		client:  &http.Client{Timeout: 120 * time.Second},
		cfg:     cfg,
	}
}

func (p *Provider) Name() string { return string(ai.ProviderOllama) }

func (p *Provider) IsAvailable() bool {
	resp, err := p.client.Get(p.baseURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func (p *Provider) Generate(ctx context.Context, req ai.GenerateRequest) (*ai.GenerateResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}

	ollamaReq := generateRequest{
		Model:  model,
		Prompt: req.Prompt,
		System: req.System,
		Stream: false,
		Options: generateOptions{
			Temperature: req.Temperature,
			TopP:        req.TopP,
			TopK:        req.TopK,
			MaxTokens:   req.MaxTokens,
		},
	}

	payload, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API error: %s", resp.Status)
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &ai.GenerateResponse{
		Content:      result.Response,
		Model:        model,
		Provider:     string(ai.ProviderOllama),
		FinishReason: "stop",
	}, nil
}

func (p *Provider) GenerateStream(ctx context.Context, req ai.GenerateRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
