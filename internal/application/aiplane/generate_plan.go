package aiplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type VectorStore interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

type SearchResult struct {
	ID       string
	Content  string
	Score    float64
	Metadata map[string]interface{}
}

type GeneratePlanUseCase struct {
	planner     attackplan.AIPlanner
	planRepo    attackplan.AttackPlanRepository
	llmClient   LLMClient
	vectorStore VectorStore
	eventStore  EventStore
}

func NewGeneratePlanUseCase(
	planner attackplan.AIPlanner,
	planRepo attackplan.AttackPlanRepository,
	llmClient LLMClient,
	vectorStore VectorStore,
	eventStore EventStore,
) *GeneratePlanUseCase {
	return &GeneratePlanUseCase{
		planner:     planner,
		planRepo:    planRepo,
		llmClient:   llmClient,
		vectorStore: vectorStore,
		eventStore:  eventStore,
	}
}
