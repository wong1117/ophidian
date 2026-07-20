package ai

import "fmt"

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
