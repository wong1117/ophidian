package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type usageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   usageInfo    `json:"usage"`
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
		baseURL = "https://api.openai.com/v1"
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

func (p *Provider) Name() string { return string(ai.ProviderOpenAI) }

func (p *Provider) IsAvailable() bool {
	return p.apiKey != ""
}

func (p *Provider) Generate(ctx context.Context, req ai.GenerateRequest) (*ai.GenerateResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}

	messages := []chatMessage{}
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.Prompt})

	body := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"top_p":       req.TopP,
	}

	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error: %s", resp.Status)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return &ai.GenerateResponse{
		Content:  result.Choices[0].Message.Content,
		Model:    result.Model,
		Provider: string(ai.ProviderOpenAI),
		TokenUsage: ai.TokenUsage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
		FinishReason: result.Choices[0].FinishReason,
	}, nil
}

func (p *Provider) GenerateStream(ctx context.Context, req ai.GenerateRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
