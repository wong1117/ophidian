package providerfactory

import (
	"context"

	"github.com/ophidian/ophidian/internal/infrastructure/ai"
)

type LLMClientAdapter struct {
	provider ai.Provider
}

func NewLLMClientAdapter(provider ai.Provider) *LLMClientAdapter {
	return &LLMClientAdapter{provider: provider}
}

func (a *LLMClientAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := a.provider.Generate(ctx, ai.GenerateRequest{
		Prompt: prompt,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}
