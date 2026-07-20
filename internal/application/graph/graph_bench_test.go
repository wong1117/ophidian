package graph

import (
	"context"
	"fmt"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainGraph "github.com/ophidian/ophidian/internal/domain/graph"
)

func BenchmarkGraphService_AddNode(b *testing.B) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.AddNode(ctx, "g1", "mission", fmt.Sprintf("m%d", i), nil, nil)
	}
}

func BenchmarkGraphService_AddEdge(b *testing.B) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)
	ctx := context.Background()

	svc.AddNode(ctx, "g1", "a", "a", nil, nil)
	svc.AddNode(ctx, "g1", "b", "b", nil, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.AddEdge(ctx, "g1", "a", "b", "TO", 1.0, nil)
	}
}

func BenchmarkGraphService_ShortestPath_10Nodes(b *testing.B) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		repo.nodes[fmt.Sprintf("n%d", i)] = &domainGraph.Node{ID: common.ID(fmt.Sprintf("n%d", i)), EntityType: "t"}
	}
	for i := 0; i < 9; i++ {
		svc.AddEdge(ctx, "g1", fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i+1), "NEXT", 1, nil)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.ShortestPath(ctx, "n0", "n9")
	}
}

func BenchmarkGraphService_Traverse_10Nodes(b *testing.B) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		repo.nodes[fmt.Sprintf("n%d", i)] = &domainGraph.Node{ID: common.ID(fmt.Sprintf("n%d", i)), EntityType: "t"}
	}
	for i := 0; i < 9; i++ {
		svc.AddEdge(ctx, "g1", fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i+1), "NEXT", 1, nil)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.Traverse(ctx, "n0", 10)
	}
}
