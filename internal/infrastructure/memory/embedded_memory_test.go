package memory

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/stretchr/testify/assert"
)

func TestEmbeddedMemory_Add(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	err := em.Add(ctx, &MemoryEntry{
		Type:      TypeConversation,
		Content:   "Hello, this is a test conversation",
		MissionID: "mission-1",
		Tags:      []string{"greeting", "test"},
	})

	assert.NoError(t, err)
	stats := em.Stats()
	assert.Equal(t, 1, stats.Total)
	assert.Equal(t, 1, stats.Conversations)
}

func TestEmbeddedMemory_Search(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	em.Add(ctx, &MemoryEntry{Type: TypeEvidence, Content: "SQL injection vulnerability found in login form", MissionID: "m1"})
	em.Add(ctx, &MemoryEntry{Type: TypeEvidence, Content: "Cross-site scripting found in comment section", MissionID: "m1"})
	em.Add(ctx, &MemoryEntry{Type: TypeEvidence, Content: "Server misconfiguration detected on port 443", MissionID: "m2"})

	results, err := em.Search(ctx, "SQL injection", 3)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Greater(t, results[0].Similarity, results[1].Similarity)
}

func TestEmbeddedMemory_SearchWithFilter(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	em.Add(ctx, &MemoryEntry{Type: TypeMission, Content: "Mission Alpha recon phase", MissionID: "alpha", Tags: []string{"recon"}})
	em.Add(ctx, &MemoryEntry{Type: TypeMission, Content: "Mission Beta recon phase", MissionID: "beta", Tags: []string{"recon"}})
	em.Add(ctx, &MemoryEntry{Type: TypeConversation, Content: "Operator chat about recon", MissionID: "chat", Tags: []string{"chat"}})

	results, err := em.SearchWithFilter(ctx, "recon", 5, TypeMission, "alpha", nil)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, TypeMission, results[0].Entry.Type)
}

func TestEmbeddedMemory_GetByMission(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	em.Add(ctx, &MemoryEntry{Type: TypeMission, Content: "Entry 1", MissionID: "m1"})
	em.Add(ctx, &MemoryEntry{Type: TypeMission, Content: "Entry 2", MissionID: "m1"})
	em.Add(ctx, &MemoryEntry{Type: TypeEvidence, Content: "Entry 3", MissionID: "m2"})

	entries, err := em.GetByMission(ctx, "m1")
	assert.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestEmbeddedMemory_GetByType(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	em.Add(ctx, &MemoryEntry{Type: TypeConversation, Content: "Chat 1"})
	em.Add(ctx, &MemoryEntry{Type: TypeConversation, Content: "Chat 2"})
	em.Add(ctx, &MemoryEntry{Type: TypeSystem, Content: "System log"})

	entries, err := em.GetByType(ctx, TypeConversation)
	assert.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestEmbeddedMemory_Delete(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	entry := &MemoryEntry{ID: common.NewID(), Type: TypeContext, Content: "temp"}
	em.Add(ctx, entry)
	err := em.Delete(ctx, entry.ID.String())
	assert.NoError(t, err)
	assert.Equal(t, 0, em.Stats().Total)
}

func TestEmbeddedMemory_Cleanup(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	expired := time.Now().Add(-time.Hour)
	em.Add(ctx, &MemoryEntry{Type: TypeSystem, Content: "old", ExpiresAt: &expired})
	em.Add(ctx, &MemoryEntry{Type: TypeSystem, Content: "new"})

	deleted, err := em.Cleanup(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, deleted)
	assert.Equal(t, 1, em.Stats().Total)
}

func TestEmbeddedMemory_AddBatch(t *testing.T) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	entries := []*MemoryEntry{
		{Type: TypeContext, Content: "ctx1"},
		{Type: TypeContext, Content: "ctx2"},
		{Type: TypeContext, Content: "ctx3"},
	}

	err := em.AddBatch(ctx, entries)
	assert.NoError(t, err)
	assert.Equal(t, 3, em.Stats().Total)
}

func TestEmbeddedMemory_Persistence(t *testing.T) {
	path := "/tmp/test-memory-persist.json"
	defer os.Remove(path)

	em1 := NewEmbeddedMemory(Config{Dimension: 128, PersistPath: path})
	ctx := context.Background()
	em1.Add(ctx, &MemoryEntry{Type: TypeMission, Content: "persisted entry", MissionID: "p1"})
	em1.Persist()

	em2 := NewEmbeddedMemory(Config{Dimension: 128, PersistPath: path})
	assert.Equal(t, 1, em2.Stats().Total)
	entries, _ := em2.GetByMission(ctx, "p1")
	assert.Len(t, entries, 1)
	assert.Equal(t, "persisted entry", entries[0].Content)
}

func TestEmbeddedMemory_Embedding_DifferentTexts(t *testing.T) {
	e1 := embed("SQL injection attack", 128)
	e2 := embed("Cross-site scripting", 128)

	sim := cosineSimilarity(e1, e2)
	assert.Less(t, sim, 0.99, "different texts should have somewhat dissimilar embeddings")

	e3 := embed("SQL injection attack", 128)
	sim2 := cosineSimilarity(e1, e3)
	assert.Greater(t, sim2, 0.99, "identical texts should be very similar")
}

func TestCosineSimilarity(t *testing.T) {
	a := Embedding{1, 0, 0}
	b := Embedding{0, 1, 0}
	assert.InDelta(t, 0.0, cosineSimilarity(a, b), 0.01)

	c := Embedding{1, 0, 0}
	assert.InDelta(t, 1.0, cosineSimilarity(a, c), 0.01)

	assert.Equal(t, 0.0, cosineSimilarity(nil, nil))
	assert.Equal(t, 0.0, cosineSimilarity(a, Embedding{1, 0}))
}

func BenchmarkEmbeddedMemory_Add(b *testing.B) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.Add(ctx, &MemoryEntry{
			Type:    TypeConversation,
			Content: "The quick brown fox jumps over the lazy dog. This is a sample text for benchmarking embedding generation.",
		})
	}
}

func BenchmarkEmbeddedMemory_Search(b *testing.B) {
	em := NewEmbeddedMemory(DefaultConfig())
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		em.Add(ctx, &MemoryEntry{
			Type:    TypeConversation,
			Content: "Sample benchmark entry for search testing with varied content to make it realistic",
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.Search(ctx, "benchmark query text", 10)
	}
}

func BenchmarkEmbed_128(b *testing.B) {
	text := "A comprehensive security assessment revealed multiple vulnerabilities in the target infrastructure"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		embed(text, 128)
	}
}

func BenchmarkEmbed_256(b *testing.B) {
	text := "A comprehensive security assessment revealed multiple vulnerabilities in the target infrastructure"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		embed(text, 256)
	}
}
