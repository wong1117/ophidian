package ai

import "context"

type ProviderType string

const (
	ProviderOllama    ProviderType = "ollama"
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderGemini    ProviderType = "gemini"
)

type Provider interface {
	Name() string
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
	GenerateStream(ctx context.Context, req GenerateRequest) (<-chan string, error)
	IsAvailable() bool
}

type GenerateRequest struct {
	Model       string
	Prompt      string
	System      string
	Temperature float64
	MaxTokens   int
	TopP        float64
	TopK        int
}

type GenerateResponse struct {
	Content      string
	Model        string
	Provider     string
	TokenUsage   TokenUsage
	FinishReason string
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type ProviderConfig struct {
	Type        ProviderType
	APIKey      string
	Model       string
	BaseURL     string
	MaxTokens   int
	Temperature float64
	TopP        float64
	Timeout     int
}
