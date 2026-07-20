package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type Embedding []float32

type MemoryEntry struct {
	ID         common.ID
	Type       EntryType
	Content    string
	Embedding  Embedding
	MissionID  string
	Tags       []string
	Metadata   map[string]interface{}
	Confidence float64
	CreatedAt  time.Time
	ExpiresAt  *time.Time
}

type EntryType string

const (
	TypeConversation EntryType = "CONVERSATION"
	TypeMission      EntryType = "MISSION"
	TypeEvidence     EntryType = "EVIDENCE"
	TypeContext      EntryType = "CONTEXT"
	TypeSystem       EntryType = "SYSTEM"
)

type SearchResult struct {
	Entry      *MemoryEntry
	Similarity float64
}

type EmbeddedMemory struct {
	mu        sync.RWMutex
	entries   map[string]*MemoryEntry
	dimension int
	persistPath string
	dirty     bool
}

type Config struct {
	Dimension   int
	PersistPath string
}

func DefaultConfig() Config {
	return Config{
		Dimension:   128,
		PersistPath: "",
	}
}

func NewEmbeddedMemory(cfg Config) *EmbeddedMemory {
	em := &EmbeddedMemory{
		entries:     make(map[string]*MemoryEntry),
		dimension:   cfg.Dimension,
		persistPath: cfg.PersistPath,
	}
	if cfg.PersistPath != "" {
		em.load()
	}
	return em
}

func (em *EmbeddedMemory) Add(ctx context.Context, entry *MemoryEntry) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if entry.ID.IsZero() {
		entry.ID = common.NewID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if len(entry.Embedding) == 0 {
		entry.Embedding = embed(entry.Content, em.dimension)
	}

	em.entries[entry.ID.String()] = entry
	em.dirty = true
	return em.maybePersist()
}

func (em *EmbeddedMemory) AddBatch(ctx context.Context, entries []*MemoryEntry) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	for _, entry := range entries {
		if entry.ID.IsZero() {
			entry.ID = common.NewID()
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = time.Now()
		}
		if len(entry.Embedding) == 0 {
			entry.Embedding = embed(entry.Content, em.dimension)
		}
		em.entries[entry.ID.String()] = entry
	}
	em.dirty = true
	return em.maybePersist()
}

func (em *EmbeddedMemory) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	queryEmbedding := embed(query, em.dimension)

	em.mu.RLock()
	defer em.mu.RUnlock()

	return em.searchSimilar(queryEmbedding, limit, nil)
}

func (em *EmbeddedMemory) SearchWithFilter(ctx context.Context, query string, limit int, entryType EntryType, missionID string, tags []string) ([]SearchResult, error) {
	queryEmbedding := embed(query, em.dimension)

	em.mu.RLock()
	defer em.mu.RUnlock()

	filter := func(e *MemoryEntry) bool {
		if entryType != "" && e.Type != entryType {
			return false
		}
		if missionID != "" && e.MissionID != missionID {
			return false
		}
		if len(tags) > 0 && !hasAnyTag(e.Tags, tags) {
			return false
		}
		return true
	}

	return em.searchSimilar(queryEmbedding, limit, filter)
}

func (em *EmbeddedMemory) searchSimilar(query Embedding, limit int, filter func(*MemoryEntry) bool) ([]SearchResult, error) {
	var results []SearchResult

	for _, entry := range em.entries {
		if filter != nil && !filter(entry) {
			continue
		}
		if len(entry.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(query, entry.Embedding)
		results = append(results, SearchResult{Entry: entry, Similarity: sim})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (em *EmbeddedMemory) GetByID(ctx context.Context, id string) (*MemoryEntry, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	entry, ok := em.entries[id]
	if !ok {
		return nil, fmt.Errorf("memory entry not found: %s", id)
	}
	return entry, nil
}

func (em *EmbeddedMemory) GetByMission(ctx context.Context, missionID string) ([]*MemoryEntry, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var result []*MemoryEntry
	for _, e := range em.entries {
		if e.MissionID == missionID {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (em *EmbeddedMemory) GetByType(ctx context.Context, entryType EntryType) ([]*MemoryEntry, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var result []*MemoryEntry
	for _, e := range em.entries {
		if e.Type == entryType {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (em *EmbeddedMemory) Delete(ctx context.Context, id string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	delete(em.entries, id)
	em.dirty = true
	return em.maybePersist()
}

func (em *EmbeddedMemory) Cleanup(ctx context.Context) (int, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	now := time.Now()
	deleted := 0
	for id, entry := range em.entries {
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			delete(em.entries, id)
			deleted++
		}
		continue
	}
	if deleted > 0 {
		em.dirty = true
		_ = em.maybePersist()
	}
	return deleted, nil
}

func (em *EmbeddedMemory) Stats() MemoryStats {
	em.mu.RLock()
	defer em.mu.RUnlock()

	stats := MemoryStats{Total: len(em.entries)}
	for _, e := range em.entries {
		switch e.Type {
		case TypeConversation:
			stats.Conversations++
		case TypeMission:
			stats.Missions++
		case TypeEvidence:
			stats.Evidence++
		case TypeContext:
			stats.Context++
		case TypeSystem:
			stats.System++
		}
	}
	return stats
}

type MemoryStats struct {
	Total         int `json:"total"`
	Conversations int `json:"conversations"`
	Missions      int `json:"missions"`
	Evidence      int `json:"evidence"`
	Context       int `json:"context"`
	System        int `json:"system"`
}

func (em *EmbeddedMemory) load() {
	data, err := os.ReadFile(em.persistPath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &em.entries)
}

func (em *EmbeddedMemory) maybePersist() error {
	if em.persistPath == "" || !em.dirty {
		return nil
	}
	data, err := json.Marshal(em.entries)
	if err != nil {
		return fmt.Errorf("persist memory: %w", err)
	}
	if err := os.WriteFile(em.persistPath, data, 0644); err != nil {
		return fmt.Errorf("persist memory write: %w", err)
	}
	em.dirty = false
	return nil
}

func (em *EmbeddedMemory) Persist() error {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.dirty = true
	return em.maybePersist()
}

func embed(text string, dim int) Embedding {
	vec := make(Embedding, dim)
	if text == "" {
		return vec
	}

	lower := strings.ToLower(text)
	for i, c := range lower {
		idx := i % dim
		vec[idx] += float32(c) * 0.01
	}

	for i := 0; i < dim; i++ {
		phase := float64(i) * 0.1
		vec[i] += float32(math.Sin(phase+float64(vec[i])) * 0.1)
	}

	norm := float32(0)
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		norm = 1.0 / float32(math.Sqrt(float64(norm)))
		for i := range vec {
			vec[i] *= norm
		}
	}

	return vec
}

func cosineSimilarity(a, b Embedding) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		va := float64(a[i])
		vb := float64(b[i])
		dot += va * vb
		normA += va * va
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
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
