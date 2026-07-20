package vectorstore

import (
	"context"
	"fmt"

	"github.com/ophidian/ophidian/internal/application/aiplane"
	"github.com/ophidian/ophidian/internal/infrastructure/ai/embedding"
)

type Adapter struct {
	store    *QdrantStore
	embedder *embedding.Generator
}

func NewAdapter(store *QdrantStore, embedder *embedding.Generator) *Adapter {
	return &Adapter{
		store:    store,
		embedder: embedder,
	}
}

func (a *Adapter) Search(ctx context.Context, query string, limit int) ([]aiplane.SearchResult, error) {
	if query == "" {
		return nil, nil
	}

	vec, err := a.embedder.Generate(query)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	results, err := a.store.Search(vec, limit)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	mapped := make([]aiplane.SearchResult, len(results))
	for i, r := range results {
		content := ""
		if c, ok := r.Payload["content"].(string); ok {
			content = c
		}
		mapped[i] = aiplane.SearchResult{
			ID:       r.ID,
			Content:  content,
			Score:    r.Score,
			Metadata: r.Payload,
		}
	}

	return mapped, nil
}
