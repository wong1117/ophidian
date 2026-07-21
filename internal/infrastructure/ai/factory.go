package ai

import (
	"context"
	"fmt"
)

func NewProviderFromConfig(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case ProviderOllama:
		return newOllamaProvider(cfg), nil
	case ProviderOpenAI:
		return newOpenAIProvider(cfg), nil
	case ProviderAnthropic:
		return newAnthropicProvider(cfg), nil
	case ProviderGemini:
		return newGeminiProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

func newOllamaProvider(cfg ProviderConfig) Provider {
	return &ollamaProvider{cfg: cfg}
}

func newOpenAIProvider(cfg ProviderConfig) Provider {
	return &openAIProvider{cfg: cfg}
}

func newAnthropicProvider(cfg ProviderConfig) Provider {
	return &anthropicProvider{cfg: cfg}
}

func newGeminiProvider(cfg ProviderConfig) Provider {
	return &geminiProvider{cfg: cfg}
}

type ollamaProvider struct {
	cfg ProviderConfig
}

func (p *ollamaProvider) Name() string                                       { return string(ProviderOllama) }
func (p *ollamaProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	return nil, fmt.Errorf("ollama provider not implemented")
}
func (p *ollamaProvider) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan string, error) {
	return nil, fmt.Errorf("ollama stream not implemented")
}
func (p *ollamaProvider) IsAvailable() bool { return false }

type openAIProvider struct {
	cfg ProviderConfig
}

func (p *openAIProvider) Name() string { return string(ProviderOpenAI) }
func (p *openAIProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	return nil, fmt.Errorf("openai provider not implemented")
}
func (p *openAIProvider) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan string, error) {
	return nil, fmt.Errorf("openai stream not implemented")
}
func (p *openAIProvider) IsAvailable() bool { return false }

type anthropicProvider struct {
	cfg ProviderConfig
}

func (p *anthropicProvider) Name() string { return string(ProviderAnthropic) }
func (p *anthropicProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	return nil, fmt.Errorf("anthropic provider not implemented")
}
func (p *anthropicProvider) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan string, error) {
	return nil, fmt.Errorf("anthropic stream not implemented")
}
func (p *anthropicProvider) IsAvailable() bool { return false }

type geminiProvider struct {
	cfg ProviderConfig
}

func (p *geminiProvider) Name() string { return string(ProviderGemini) }
func (p *geminiProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	return nil, fmt.Errorf("gemini provider not implemented")
}
func (p *geminiProvider) GenerateStream(ctx context.Context, req GenerateRequest) (<-chan string, error) {
	return nil, fmt.Errorf("gemini stream not implemented")
}
func (p *geminiProvider) IsAvailable() bool { return false }
