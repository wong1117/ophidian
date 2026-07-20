package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
)

type contentPart struct {
	Text string `json:"text"`
}

type contentEntry struct {
	Parts []contentPart `json:"parts"`
	Role  string        `json:"role"`
}

type safetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type candidate struct {
	Content      contentEntry   `json:"content"`
	FinishReason string         `json:"finishReason"`
	SafetyRatings []safetyRating `json:"safetyRatings"`
}

type tokenCount struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiResponse struct {
	Candidates []candidate `json:"candidates"`
	Usage      tokenCount  `json:"usageMetadata"`
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
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-pro"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120
	}
	return &Provider{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
		cfg:     cfg,
	}
}

func (p *Provider) Name() string { return string(ai.ProviderGemini) }

func (p *Provider) IsAvailable() bool {
	return p.apiKey != ""
}

func (p *Provider) Generate(ctx context.Context, req ai.GenerateRequest) (*ai.GenerateResponse, error) {
	contents := []contentEntry{}
	if req.System != "" {
		contents = append(contents, contentEntry{
			Role:  "user",
			Parts: []contentPart{{Text: req.System + "\n\n" + req.Prompt}},
		})
	} else {
		contents = append(contents, contentEntry{
			Role:  "user",
			Parts: []contentPart{{Text: req.Prompt}},
		})
	}

	body := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     req.Temperature,
			"maxOutputTokens": req.MaxTokens,
			"topP":            req.TopP,
			"topK":            req.TopK,
		},
	}

	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
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
		return nil, fmt.Errorf("gemini API error: %s", resp.Status)
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	return &ai.GenerateResponse{
		Content:  result.Candidates[0].Content.Parts[0].Text,
		Model:    p.model,
		Provider: string(ai.ProviderGemini),
		TokenUsage: ai.TokenUsage{
			PromptTokens:     result.Usage.PromptTokenCount,
			CompletionTokens: result.Usage.CandidatesTokenCount,
			TotalTokens:      result.Usage.TotalTokenCount,
		},
		FinishReason: result.Candidates[0].FinishReason,
	}, nil
}

func (p *Provider) GenerateStream(ctx context.Context, req ai.GenerateRequest) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
