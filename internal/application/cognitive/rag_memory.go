package cognitive

import (
	"context"
	"time"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type MemoryEntryType string

const (
	MemoryMissionSuccess MemoryEntryType = "MISSION_SUCCESS"
	MemoryMissionFailure MemoryEntryType = "MISSION_FAILURE"
	MemoryTechniqueOk    MemoryEntryType = "TECHNIQUE_OK"
	MemoryTechniqueFail  MemoryEntryType = "TECHNIQUE_FAIL"
	MemoryEnvironment   MemoryEntryType = "ENVIRONMENT"
	MemoryTargetProfile MemoryEntryType = "TARGET_PROFILE"
)

type MemoryEntry struct {
	ID           common.ID
	Type         MemoryEntryType
	Content      string
	Tags         []string
	MissionID    string
	TargetOS     string
	TargetEnv    string
	Technique    string
	CVE          string
	Severity     string
	Confidence   float64
	Success      bool
	Context      map[string]interface{}
	Embedding    []float32
	CreatedAt    time.Time
	ExpiresAt    *time.Time
}

type RAGStore interface {
	SaveMemory(ctx context.Context, entry *MemoryEntry) error
	SearchMemory(ctx context.Context, query string, tags []string, limit int) ([]MemoryEntry, error)
	SearchByTechnique(ctx context.Context, technique string) ([]MemoryEntry, error)
	SearchByEnvironment(ctx context.Context, os, env string) ([]MemoryEntry, error)
	GetRecentFailures(ctx context.Context, limit int) ([]MemoryEntry, error)
	GetRecentSuccesses(ctx context.Context, limit int) ([]MemoryEntry, error)
	DeleteExpired(ctx context.Context) error
}

type RAGMemory struct {
	store      RAGStore
	embedder   Embedder
	contextBuf []MemoryEntry
}

func NewRAGMemory(store RAGStore, embedder Embedder) *RAGMemory {
	return &RAGMemory{
		store:      store,
		embedder:   embedder,
		contextBuf: make([]MemoryEntry, 0),
	}
}

func (m *RAGMemory) RecordMissionSuccess(ctx context.Context, missionID, technique, targetOS, targetEnv string, tags []string) error {
	entry := &MemoryEntry{
		ID:        common.NewID(),
		Type:      MemoryMissionSuccess,
		Content:   "Mission successful using technique: " + technique,
		Tags:      tags,
		MissionID: missionID,
		TargetOS:  targetOS,
		TargetEnv: targetEnv,
		Technique: technique,
		Success:   true,
		Confidence: 1.0,
		CreatedAt: time.Now(),
	}
	embedding, err := m.embedder.Embed(ctx, entry.Content)
	if err != nil {
		return err
	}
	entry.Embedding = embedding
	entry.Technique = technique
	entry.Success = true
	return m.store.SaveMemory(ctx, entry)
}

func (m *RAGMemory) RecordTechniqueFailure(ctx context.Context, missionID, technique, targetOS, targetEnv, reason string, tags []string) error {
	entry := &MemoryEntry{
		ID:        common.NewID(),
		Type:      MemoryTechniqueFail,
		Content:   "Technique failed: " + technique + " reason: " + reason,
		Tags:      tags,
		MissionID: missionID,
		TargetOS:  targetOS,
		TargetEnv: targetEnv,
		Technique: technique,
		Success:   false,
		Confidence: 0.0,
		CreatedAt: time.Now(),
	}
	embedding, err := m.embedder.Embed(ctx, entry.Content)
	if err != nil {
		return err
	}
	entry.Embedding = embedding
	return m.store.SaveMemory(ctx, entry)
}

func (m *RAGMemory) RecordEnvironment(ctx context.Context, missionID, os, env string, details map[string]interface{}) error {
	entry := &MemoryEntry{
		ID:        common.NewID(),
		Type:      MemoryEnvironment,
		Content:   "Environment: " + os + "/" + env,
		Tags:      []string{os, env},
		MissionID: missionID,
		TargetOS:  os,
		TargetEnv: env,
		Context:   details,
		CreatedAt: time.Now(),
	}
	embedding, err := m.embedder.Embed(ctx, entry.Content)
	if err != nil {
		return err
	}
	entry.Embedding = embedding
	return m.store.SaveMemory(ctx, entry)
}

func (m *RAGMemory) GetContextForPlanning(ctx context.Context, targetOS, targetEnv string, plannedTechnique string) (string, error) {
	entries := make([]MemoryEntry, 0)

	envResults, err := m.store.SearchByEnvironment(ctx, targetOS, targetEnv)
	if err == nil {
		entries = append(entries, envResults...)
	}

	techResults, err := m.store.SearchByTechnique(ctx, plannedTechnique)
	if err == nil {
		entries = append(entries, techResults...)
	}

	recentFailures, err := m.store.GetRecentFailures(ctx, 5)
	if err == nil {
		entries = append(entries, recentFailures...)
	}

	recentSuccesses, err := m.store.GetRecentSuccesses(ctx, 5)
	if err == nil {
		entries = append(entries, recentSuccesses...)
	}

	contextStr := "Previous mission context:\n"
	for _, e := range entries {
		status := "SUCCESS"
		if !e.Success {
			status = "FAILURE"
		}
		contextStr += "- [" + status + "] " + e.Content + "\n"
	}

	return contextStr, nil
}
