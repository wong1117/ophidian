package providerfactory

import (
	"fmt"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
	"github.com/ophidian/ophidian/internal/infrastructure/ai/anthropic"
	"github.com/ophidian/ophidian/internal/infrastructure/ai/google"
	"github.com/ophidian/ophidian/internal/infrastructure/ai/ollama"
	"github.com/ophidian/ophidian/internal/infrastructure/ai/openai"
)

func NewProviderFromConfig(cfg ai.ProviderConfig) (ai.Provider, error) {
	switch cfg.Type {
	case ai.ProviderOllama:
		return ollama.NewProvider(cfg), nil
	case ai.ProviderOpenAI:
		return openai.NewProvider(cfg), nil
	case ai.ProviderAnthropic:
		return anthropic.NewProvider(cfg), nil
	case ai.ProviderGemini:
		return google.NewProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}
