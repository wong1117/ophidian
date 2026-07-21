package rag

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type VectorStorePort interface {
	Store(ctx context.Context, id string, vector []float32, payload map[string]interface{}) error
	Search(ctx context.Context, vector []float32, limit int) ([]VectorResult, error)
}

type VectorResult struct {
	ID      string
	Score   float64
	Payload map[string]interface{}
}

type AIMemoryService struct {
	embedder common.Embedder
	vectors  VectorStorePort

	mu      sync.RWMutex
	entries map[string]*common.MemoryEntry
}

func NewAIMemoryService(embedder common.Embedder, vectors VectorStorePort) *AIMemoryService {
	return &AIMemoryService{
		embedder: embedder,
		vectors:  vectors,
		entries:  make(map[string]*common.MemoryEntry),
	}
}

func (s *AIMemoryService) SaveMemory(ctx context.Context, entry *common.MemoryEntry) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save memory: %w", err)
	}

	if entry.ID.IsZero() {
		entry.ID = common.NewID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	embedding, err := s.embedder.Embed(ctx, entry.Content)
	if err != nil {
		return fmt.Errorf("save memory: embed: %w", err)
	}
	entry.Embedding = embedding

	payload := buildPayload(entry)
	if err := s.vectors.Store(ctx, entry.ID.String(), embedding, payload); err != nil {
		return fmt.Errorf("save memory: store vector: %w", err)
	}

	s.mu.Lock()
	s.entries[entry.ID.String()] = entry
	s.mu.Unlock()

	return nil
}

func (s *AIMemoryService) SearchMemory(ctx context.Context, query string, tags []string, limit int) ([]common.MemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("search memory: %w", err)
	}

	if limit <= 0 {
		limit = 10
	}

	if query == "" {
		return s.searchByTags(ctx, tags, limit)
	}

	queryVec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search memory: embed query: %w", err)
	}

	results, err := s.vectors.Search(ctx, queryVec, limit*2)
	if err != nil {
		return nil, fmt.Errorf("search memory: vector search: %w", err)
	}

	ranked := s.rankBySimilarity(queryVec, results)
	filtered := s.filterByTags(ranked, tags)

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	entries := make([]common.MemoryEntry, 0, len(filtered))
	for _, r := range filtered {
		e := s.mapResultToEntry(r)
		if e != nil {
			entries = append(entries, *e)
		}
	}

	return entries, nil
}

func (s *AIMemoryService) SearchByTechnique(ctx context.Context, technique string) ([]common.MemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("search by technique: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []common.MemoryEntry
	for _, e := range s.entries {
		if strings.EqualFold(e.Technique, technique) {
			results = append(results, *e)
		}
	}
	return results, nil
}

func (s *AIMemoryService) SearchByEnvironment(ctx context.Context, os, env string) ([]common.MemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("search by environment: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []common.MemoryEntry
	for _, e := range s.entries {
		if strings.EqualFold(e.TargetOS, os) && strings.EqualFold(e.TargetEnv, env) {
			results = append(results, *e)
		}
	}
	return results, nil
}

func (s *AIMemoryService) GetRecentFailures(ctx context.Context, limit int) ([]common.MemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get recent failures: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.sortedByTime()
	var results []common.MemoryEntry
	for _, e := range all {
		if !e.Success && len(results) < limit {
			results = append(results, *e)
		}
	}
	return results, nil
}

func (s *AIMemoryService) GetRecentSuccesses(ctx context.Context, limit int) ([]common.MemoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get recent successes: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.sortedByTime()
	var results []common.MemoryEntry
	for _, e := range all {
		if e.Success && len(results) < limit {
			results = append(results, *e)
		}
	}
	return results, nil
}

func (s *AIMemoryService) DeleteExpired(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete expired: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, e := range s.entries {
		if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
			delete(s.entries, id)
		}
	}
	return nil
}

func (s *AIMemoryService) rankBySimilarity(queryVec []float32, results []VectorResult) []VectorResult {
	scored := make([]VectorResult, len(results))
	copy(scored, results)

	for i := range scored {
		s.mu.RLock()
		entry, ok := s.entries[scored[i].ID]
		s.mu.RUnlock()
		if ok && len(entry.Embedding) > 0 {
			scored[i].Score = s.embedder.Similarity(queryVec, entry.Embedding)
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	return scored
}

func (s *AIMemoryService) filterByTags(results []VectorResult, tags []string) []VectorResult {
	if len(tags) == 0 {
		return results
	}
	var filtered []VectorResult
	for _, r := range results {
		s.mu.RLock()
		entry, ok := s.entries[r.ID]
		s.mu.RUnlock()
		if ok && hasAnyTag(entry.Tags, tags) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (s *AIMemoryService) searchByTags(ctx context.Context, tags []string, limit int) ([]common.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.sortedByTime()
	var results []common.MemoryEntry
	for _, e := range all {
		if len(tags) == 0 || hasAnyTag(e.Tags, tags) {
			results = append(results, *e)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (s *AIMemoryService) sortedByTime() []*common.MemoryEntry {
	all := make([]*common.MemoryEntry, 0, len(s.entries))
	for _, e := range s.entries {
		all = append(all, e)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all
}

func (s *AIMemoryService) mapResultToEntry(result VectorResult) *common.MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[result.ID]
	if !ok {
		return nil
	}
	cpy := *e
	cpy.Embedding = nil
	return &cpy
}

func buildPayload(entry *common.MemoryEntry) map[string]interface{} {
	p := map[string]interface{}{
		"type":       string(entry.Type),
		"content":    entry.Content,
		"mission_id": entry.MissionID,
		"technique":  entry.Technique,
		"cve":        entry.CVE,
		"severity":   entry.Severity,
		"confidence": entry.Confidence,
		"success":    entry.Success,
	}
	if len(entry.Tags) > 0 {
		p["tags"] = entry.Tags
	}
	if entry.Context != nil {
		for k, v := range entry.Context {
			p[k] = v
		}
	}
	return p
}

func hasAnyTag(entryTags, queryTags []string) bool {
	for _, qt := range queryTags {
		for _, et := range entryTags {
			if strings.EqualFold(et, qt) {
				return true
			}
		}
	}
	return false
}
