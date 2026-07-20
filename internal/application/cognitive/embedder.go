package cognitive

import (
	"context"
	"math"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Similarity(a, b []float32) float64
}

type SimpleEmbedder struct{}

func NewSimpleEmbedder() *SimpleEmbedder {
	return &SimpleEmbedder{}
}

func (e *SimpleEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	hash := int64(0)
	for _, c := range text {
		hash = hash*31 + int64(c)
	}
	dim := 32
	vector := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vector[i] = float32((hash >> (i * 2)) & 3)
		if i%2 == 0 {
			vector[i] = -vector[i]
		}
	}
	norm := float32(0)
	for _, v := range vector {
		norm += v * v
	}
	norm = 1.0 / (norm + 1e-10)
	for i := range vector {
		vector[i] *= norm
	}
	return vector, nil
}

func (e *SimpleEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		result[i] = emb
	}
	return result, nil
}

func (e *SimpleEmbedder) Similarity(a, b []float32) float64 {
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
