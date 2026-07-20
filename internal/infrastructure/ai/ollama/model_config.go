package ollama

type ModelConfig struct {
	Reasoning   string  `yaml:"reasoning"`
	Code        string  `yaml:"code"`
	Embedding   string  `yaml:"embedding"`
	Timeout     int     `yaml:"timeout"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
	TopP        float64 `yaml:"top_p"`
	TopK        int     `yaml:"top_k"`
}

func DefaultModelConfig() ModelConfig {
	return ModelConfig{
		Reasoning:   "mixtral:8x7b",
		Code:        "codellama:34b",
		Embedding:   "nomic-embed-text",
		Timeout:     120,
		MaxTokens:   4096,
		Temperature: 0.3,
		TopP:        0.9,
		TopK:        40,
	}
}
