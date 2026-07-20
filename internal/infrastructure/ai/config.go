package ai

type MultiProviderConfig struct {
	Default    ProviderType      `yaml:"default"`
	Providers  []ProviderConfig  `yaml:"providers"`
	Ollama     *ProviderConfig   `yaml:"ollama,omitempty"`
	OpenAI     *ProviderConfig   `yaml:"openai,omitempty"`
	Anthropic  *ProviderConfig   `yaml:"anthropic,omitempty"`
	Gemini     *ProviderConfig   `yaml:"gemini,omitempty"`
	Fallback   bool              `yaml:"fallback"`
}

type ProviderConfig struct {
	Type        ProviderType `yaml:"type"`
	APIKey      string       `yaml:"api_key"`
	Model       string       `yaml:"model"`
	BaseURL     string       `yaml:"base_url"`
	MaxTokens   int          `yaml:"max_tokens"`
	Temperature float64      `yaml:"temperature"`
	TopP        float64      `yaml:"top_p"`
	Timeout     int          `yaml:"timeout"`
	Priority    int          `yaml:"priority"`
}

func (c *MultiProviderConfig) GetActiveProviders() []ProviderConfig {
	var active []ProviderConfig

	for _, p := range c.Providers {
		if p.APIKey != "" || (p.Type == ProviderOllama) {
			active = append(active, p)
		}
	}

	if c.Ollama != nil {
		active = append(active, *c.Ollama)
	}
	if c.OpenAI != nil && c.OpenAI.APIKey != "" {
		active = append(active, *c.OpenAI)
	}
	if c.Anthropic != nil && c.Anthropic.APIKey != "" {
		active = append(active, *c.Anthropic)
	}
	if c.Gemini != nil && c.Gemini.APIKey != "" {
		active = append(active, *c.Gemini)
	}

	return active
}

func (c *MultiProviderConfig) HasAvailableProvider() bool {
	for _, p := range c.Providers {
		if p.APIKey != "" || p.Type == ProviderOllama {
			return true
		}
	}
	if c.Ollama != nil {
		return true
	}
	if c.OpenAI != nil && c.OpenAI.APIKey != "" {
		return true
	}
	if c.Anthropic != nil && c.Anthropic.APIKey != "" {
		return true
	}
	if c.Gemini != nil && c.Gemini.APIKey != "" {
		return true
	}
	return false
}
