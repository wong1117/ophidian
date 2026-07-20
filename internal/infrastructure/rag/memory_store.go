package rag

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/application/cognitive"
)

type MemoryStore struct {
	mu      sync.RWMutex
	entries []cognitive.MemoryEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make([]cognitive.MemoryEntry, 0),
	}
}

func (s *MemoryStore) SaveMemory(ctx context.Context, entry *cognitive.MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, *entry)
	return nil
}

func (s *MemoryStore) SearchMemory(ctx context.Context, query string, tags []string, limit int) ([]cognitive.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []cognitive.MemoryEntry
	for _, e := range s.entries {
		if !matchTags(e.Tags, tags) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Content), strings.ToLower(query)) {
			continue
		}
		results = append(results, e)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *MemoryStore) SearchByTechnique(ctx context.Context, technique string) ([]cognitive.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []cognitive.MemoryEntry
	for _, e := range s.entries {
		if strings.EqualFold(e.Technique, technique) {
			results = append(results, e)
		}
	}
	return results, nil
}

func (s *MemoryStore) SearchByEnvironment(ctx context.Context, os, env string) ([]cognitive.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []cognitive.MemoryEntry
	for _, e := range s.entries {
		if strings.EqualFold(e.TargetOS, os) && strings.EqualFold(e.TargetEnv, env) {
			results = append(results, e)
		}
	}
	return results, nil
}

func (s *MemoryStore) GetRecentFailures(ctx context.Context, limit int) ([]cognitive.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []cognitive.MemoryEntry
	for i := len(s.entries) - 1; i >= 0 && len(results) < limit; i-- {
		if !s.entries[i].Success {
			results = append(results, s.entries[i])
		}
	}
	return results, nil
}

func (s *MemoryStore) GetRecentSuccesses(ctx context.Context, limit int) ([]cognitive.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []cognitive.MemoryEntry
	for i := len(s.entries) - 1; i >= 0 && len(results) < limit; i-- {
		if s.entries[i].Success {
			results = append(results, s.entries[i])
		}
	}
	return results, nil
}

func (s *MemoryStore) DeleteExpired(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	valid := s.entries[:0]
	for _, e := range s.entries {
		if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
			continue
		}
		valid = append(valid, e)
	}
	s.entries = valid
	return nil
}

func matchTags(entryTags, queryTags []string) bool {
	if len(queryTags) == 0 {
		return true
	}
	for _, qt := range queryTags {
		for _, et := range entryTags {
			if strings.EqualFold(et, qt) {
				return true
			}
		}
	}
	return false
}
