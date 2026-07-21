package cognitive

import (
	"context"
	"time"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type RAGStore interface {
	SaveMemory(ctx context.Context, entry *common.MemoryEntry) error
	SearchMemory(ctx context.Context, query string, tags []string, limit int) ([]common.MemoryEntry, error)
	SearchByTechnique(ctx context.Context, technique string) ([]common.MemoryEntry, error)
	SearchByEnvironment(ctx context.Context, os, env string) ([]common.MemoryEntry, error)
	GetRecentFailures(ctx context.Context, limit int) ([]common.MemoryEntry, error)
	GetRecentSuccesses(ctx context.Context, limit int) ([]common.MemoryEntry, error)
	DeleteExpired(ctx context.Context) error
}

type RAGMemory struct {
	store      RAGStore
	embedder   common.Embedder
	contextBuf []common.MemoryEntry
}

func NewRAGMemory(store RAGStore, embedder common.Embedder) *RAGMemory {
	return &RAGMemory{
		store:      store,
		embedder:   embedder,
		contextBuf: make([]common.MemoryEntry, 0),
	}
}

func (m *RAGMemory) RecordMissionSuccess(ctx context.Context, missionID, technique, targetOS, targetEnv string, tags []string) error {
	entry := &common.MemoryEntry{
		ID:        common.NewID(),
		Type:      common.MemoryMissionSuccess,
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
	entry := &common.MemoryEntry{
		ID:        common.NewID(),
		Type:      common.MemoryTechniqueFail,
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
	entry := &common.MemoryEntry{
		ID:        common.NewID(),
		Type:      common.MemoryEnvironment,
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
	entries := make([]common.MemoryEntry, 0)

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
