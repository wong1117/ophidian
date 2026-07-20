package graph

import (
	"context"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type Node struct {
	ID         common.ID
	EntityType string
	EntityID   common.ID
	Labels     []string
	Properties map[string]interface{}
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Edge struct {
	ID         common.ID
	GraphID    common.ID
	FromNodeID common.ID
	ToNodeID   common.ID
	Type       string
	Weight     float64
	Properties map[string]interface{}
	CreatedAt  time.Time
}

type Graph struct {
	ID          common.ID
	Name        string
	Description string
	Version     int
	Metadata    map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type GraphRepository interface {
	SaveGraph(ctx context.Context, g *Graph) error
	FindGraphByID(ctx context.Context, id string) (*Graph, error)
	FindGraphsByName(ctx context.Context, name string) ([]*Graph, error)
	SaveNode(ctx context.Context, node *Node) error
	FindNodeByID(ctx context.Context, id string) (*Node, error)
	FindNodesByGraph(ctx context.Context, graphID string) ([]*Node, error)
	FindNodesByEntity(ctx context.Context, entityType, entityID string) ([]*Node, error)
	DeleteNode(ctx context.Context, id string) error
	SaveEdge(ctx context.Context, edge *Edge) error
	FindEdgeByID(ctx context.Context, id string) (*Edge, error)
	FindEdgesByGraph(ctx context.Context, graphID string) ([]*Edge, error)
	FindOutgoingEdges(ctx context.Context, nodeID string) ([]*Edge, error)
	FindIncomingEdges(ctx context.Context, nodeID string) ([]*Edge, error)
	DeleteEdge(ctx context.Context, id string) error
}
