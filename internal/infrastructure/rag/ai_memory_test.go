package rag

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/ophidian/ophidian/internal/application/cognitive"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/stretchr/testify/assert"
)

type mockVectorStore struct {
	mu      sync.Mutex
	entries map[string][]float32
}

func newMockVectorStore() *mockVectorStore {
	return &mockVectorStore{entries: make(map[string][]float32)}
}

func (m *mockVectorStore) Store(ctx context.Context, id string, vector []float32, payload map[string]interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[id] = vector
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, vector []float32, limit int) ([]VectorResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []VectorResult
	for id, v := range m.entries {
		sim := cosineSimilarity(vector, v)
		results = append(results, VectorResult{ID: id, Score: sim})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func cosineSimilarity(a, b []float32) float64 {
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

func setupService() (*AIMemoryService, *mockVectorStore) {
	embedder := cognitive.NewSimpleEmbedder()
	vectors := newMockVectorStore()
	return NewAIMemoryService(embedder, vectors), vectors
}

func memEntry(id, content string, tags []string, technique string, success bool) *cognitive.MemoryEntry {
	return &cognitive.MemoryEntry{
		ID:         common.ID(id),
		Type:       cognitive.MemoryTechniqueOk,
		Content:    content,
		Tags:       tags,
		Technique:  technique,
		Success:    success,
		Confidence: 0.8,
		CreatedAt:  time.Now(),
	}
}

func TestAIMemoryService_SaveMemory(t *testing.T) {
	svc, vec := setupService()
	ctx := context.Background()

	err := svc.SaveMemory(ctx, memEntry("e1", "test entry", nil, "T1003", true))

	assert.NoError(t, err)
	assert.Len(t, vec.entries, 1)
}

func TestAIMemoryService_SaveMemory_CancelledContext(t *testing.T) {
	svc, _ := setupService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.SaveMemory(ctx, memEntry("e1", "test", nil, "T1003", true))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save memory")
}

func TestAIMemoryService_SearchMemory_Semantic(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_ = svc.SaveMemory(ctx, memEntry("e1", "SQL Injection exploit on login page", nil, "T1190", true))
	_ = svc.SaveMemory(ctx, memEntry("e2", "Reverse shell obtained via Apache", nil, "T1059", true))
	_ = svc.SaveMemory(ctx, memEntry("e3", "Network scanning recon phase", nil, "T1046", true))

	results, err := svc.SearchMemory(ctx, "SQL Injection", nil, 3)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestAIMemoryService_SearchMemory_ByTags(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_ = svc.SaveMemory(ctx, memEntry("e1", "entry A", []string{"windows", "rce"}, "T1", true))
	_ = svc.SaveMemory(ctx, memEntry("e2", "entry B", []string{"linux", "priv"}, "T2", true))
	_ = svc.SaveMemory(ctx, memEntry("e3", "entry C", []string{"windows", "scan"}, "T3", true))

	results, err := svc.SearchMemory(ctx, "entry", []string{"windows"}, 10)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestAIMemoryService_SearchMemory_EmptyQuery(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_ = svc.SaveMemory(ctx, memEntry("e1", "entry A", nil, "T1", true))
	_ = svc.SaveMemory(ctx, memEntry("e2", "entry B", nil, "T2", true))

	results, err := svc.SearchMemory(ctx, "", nil, 10)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestAIMemoryService_SearchMemory_CancelledContext(t *testing.T) {
	svc, _ := setupService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.SearchMemory(ctx, "query", nil, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search memory")
}

func TestAIMemoryService_SearchByTechnique(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_ = svc.SaveMemory(ctx, memEntry("e1", "entry A", nil, "T1003", true))
	_ = svc.SaveMemory(ctx, memEntry("e2", "entry B", nil, "T1059", false))
	_ = svc.SaveMemory(ctx, memEntry("e3", "entry C", nil, "T1003", true))

	results, err := svc.SearchByTechnique(ctx, "T1003")

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestAIMemoryService_SearchByTechnique_CancelledContext(t *testing.T) {
	svc, _ := setupService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.SearchByTechnique(ctx, "T1003")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search by technique")
}

func TestAIMemoryService_SearchByEnvironment(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	e1 := memEntry("e1", "entry A", nil, "T1", true)
	e1.TargetOS = "linux"
	e1.TargetEnv = "production"
	_ = svc.SaveMemory(ctx, e1)

	e2 := memEntry("e2", "entry B", nil, "T2", true)
	e2.TargetOS = "windows"
	e2.TargetEnv = "staging"
	_ = svc.SaveMemory(ctx, e2)

	e3 := memEntry("e3", "entry C", nil, "T3", true)
	e3.TargetOS = "linux"
	e3.TargetEnv = "production"
	_ = svc.SaveMemory(ctx, e3)

	results, err := svc.SearchByEnvironment(ctx, "linux", "production")

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestAIMemoryService_GetRecentFailures(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_ = svc.SaveMemory(ctx, memEntry("e1", "success", nil, "T1", true))
	_ = svc.SaveMemory(ctx, memEntry("e2", "fail1", nil, "T2", false))
	_ = svc.SaveMemory(ctx, memEntry("e3", "fail2", nil, "T3", false))

	results, err := svc.GetRecentFailures(ctx, 10)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.False(t, results[0].Success)
	assert.False(t, results[1].Success)
}

func TestAIMemoryService_GetRecentSuccesses(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_ = svc.SaveMemory(ctx, memEntry("e1", "success1", nil, "T1", true))
	_ = svc.SaveMemory(ctx, memEntry("e2", "fail", nil, "T2", false))
	_ = svc.SaveMemory(ctx, memEntry("e3", "success2", nil, "T3", true))

	results, err := svc.GetRecentSuccesses(ctx, 10)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].Success)
	assert.True(t, results[1].Success)
}

func TestAIMemoryService_DeleteExpired(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	expired := memEntry("e1", "expired", nil, "T1", true)
	past := time.Now().Add(-1 * time.Hour)
	expired.ExpiresAt = &past
	_ = svc.SaveMemory(ctx, expired)

	_ = svc.SaveMemory(ctx, memEntry("e2", "valid", nil, "T2", true))

	err := svc.DeleteExpired(ctx)

	assert.NoError(t, err)

	results, _ := svc.SearchMemory(ctx, "valid", nil, 10)
	assert.Len(t, results, 1)
}

func TestAIMemoryService_DeleteExpired_CancelledContext(t *testing.T) {
	svc, _ := setupService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.DeleteExpired(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete expired")
}

func TestAIMemoryService_GetRecentFailures_CancelledContext(t *testing.T) {
	svc, _ := setupService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GetRecentFailures(ctx, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get recent failures")
}

func TestAIMemoryService_Ranking_BySimilarity(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_ = svc.SaveMemory(ctx, memEntry("e1", "irrelevant content about weather", nil, "T1", true))
	_ = svc.SaveMemory(ctx, memEntry("e2", "SQL Injection is a critical web vulnerability", nil, "T2", true))

	results, err := svc.SearchMemory(ctx, "SQL Injection", nil, 2)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "e2", results[0].ID.String())
}

func TestAIMemoryService_Storage_EmbeddingPersisted(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	err := svc.SaveMemory(ctx, memEntry("e1", "test content for embedding", nil, "T1", true))
	assert.NoError(t, err)

	svc.mu.RLock()
	entry := svc.entries["e1"]
	svc.mu.RUnlock()

	assert.NotNil(t, entry)
	assert.NotEmpty(t, entry.Embedding)
	assert.Len(t, entry.Embedding, 32)
}

func TestHasAnyTag(t *testing.T) {
	assert.True(t, hasAnyTag([]string{"windows", "rce"}, []string{"windows"}))
	assert.True(t, hasAnyTag([]string{"linux"}, []string{"LINUX"}))
	assert.False(t, hasAnyTag([]string{"windows"}, []string{"mac"}))
	assert.False(t, hasAnyTag([]string{}, []string{"windows"}))
}

func TestBuildPayload(t *testing.T) {
	entry := &cognitive.MemoryEntry{
		Type:       cognitive.MemoryTechniqueOk,
		Content:    "test",
		MissionID:  "m1",
		Technique:  "T1003",
		CVE:         "CVE-2024-0001",
		Severity:    "HIGH",
		Confidence: 0.9,
		Success:    true,
		Tags:       []string{"windows", "rce"},
		Context:    map[string]interface{}{"extra": "data"},
	}

	payload := buildPayload(entry)

	assert.Equal(t, "test", payload["content"])
	assert.Equal(t, "m1", payload["mission_id"])
	assert.Equal(t, "T1003", payload["technique"])
	assert.Equal(t, "CVE-2024-0001", payload["cve"])
	assert.Equal(t, 0.9, payload["confidence"])
	assert.True(t, payload["success"].(bool))
	assert.Equal(t, "data", payload["extra"])
}

func TestVectorStoreAdapter_Search(t *testing.T) {
	ctx := context.Background()
	vec := newMockVectorStore()

	vec.Store(ctx, "id1", []float32{1.0, 0.0, 0.0}, nil)
	vec.Store(ctx, "id2", []float32{0.0, 1.0, 0.0}, nil)
	vec.Store(ctx, "id3", []float32{1.0, 0.5, 0.0}, nil)

	results, err := vec.Search(ctx, []float32{1.0, 0.1, 0.0}, 2)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestVectorStoreAdapter_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	vec := newMockVectorStore()

	_, err := vec.Search(ctx, []float32{1.0}, 2)
	assert.Error(t, err)
}
