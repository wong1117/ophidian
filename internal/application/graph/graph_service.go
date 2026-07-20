package graph

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainGraph "github.com/ophidian/ophidian/internal/domain/graph"
)

type GraphService struct {
	repo domainGraph.GraphRepository
}

func NewGraphService(repo domainGraph.GraphRepository) *GraphService {
	return &GraphService{repo: repo}
}

func (s *GraphService) CreateGraph(ctx context.Context, name, description string) (*domainGraph.Graph, error) {
	g := &domainGraph.Graph{
		ID:          common.NewID(),
		Name:        name,
		Description: description,
		Version:     1,
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.SaveGraph(ctx, g); err != nil {
		return nil, fmt.Errorf("create graph: %w", err)
	}
	return g, nil
}

func (s *GraphService) AddNode(ctx context.Context, graphID, entityType, entityID string, labels []string, properties map[string]interface{}) (*domainGraph.Node, error) {
	n := &domainGraph.Node{
		ID:         common.NewID(),
		EntityType: entityType,
		EntityID:   common.ID(entityID),
		Labels:     labels,
		Properties: properties,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.SaveNode(ctx, n); err != nil {
		return nil, fmt.Errorf("add node: %w", err)
	}
	return n, nil
}

func (s *GraphService) AddEdge(ctx context.Context, graphID, fromNodeID, toNodeID, edgeType string, weight float64, properties map[string]interface{}) (*domainGraph.Edge, error) {
	e := &domainGraph.Edge{
		ID:         common.NewID(),
		GraphID:    common.ID(graphID),
		FromNodeID: common.ID(fromNodeID),
		ToNodeID:   common.ID(toNodeID),
		Type:       edgeType,
		Weight:     weight,
		Properties: properties,
		CreatedAt:  time.Now(),
	}
	if err := s.repo.SaveEdge(ctx, e); err != nil {
		return nil, fmt.Errorf("add edge: %w", err)
	}
	return e, nil
}

func (s *GraphService) DeleteNode(ctx context.Context, nodeID string) error {
	outgoing, _ := s.repo.FindOutgoingEdges(ctx, nodeID)
	for _, e := range outgoing {
		_ = s.repo.DeleteEdge(ctx, e.ID.String())
	}
	incoming, _ := s.repo.FindIncomingEdges(ctx, nodeID)
	for _, e := range incoming {
		_ = s.repo.DeleteEdge(ctx, e.ID.String())
	}
	if err := s.repo.DeleteNode(ctx, nodeID); err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	return nil
}

func (s *GraphService) DeleteEdge(ctx context.Context, edgeID string) error {
	if err := s.repo.DeleteEdge(ctx, edgeID); err != nil {
		return fmt.Errorf("delete edge: %w", err)
	}
	return nil
}

func (s *GraphService) GetNodeByEntity(ctx context.Context, entityType, entityID string) (*domainGraph.Node, error) {
	nodes, err := s.repo.FindNodesByEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("get node by entity: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("node not found for %s/%s", entityType, entityID)
	}
	return nodes[0], nil
}

func (s *GraphService) Traverse(ctx context.Context, startNodeID string, maxDepth int) ([]*domainGraph.Node, error) {
	if maxDepth <= 0 {
		maxDepth = 10
	}

	visited := make(map[string]bool)
	var result []*domainGraph.Node
	queue := []string{startNodeID}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var nextQueue []string
		for _, nodeID := range queue {
			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true

			node, err := s.repo.FindNodeByID(ctx, nodeID)
			if err != nil {
				continue
			}
			result = append(result, node)

			edges, err := s.repo.FindOutgoingEdges(ctx, nodeID)
			if err != nil {
				continue
			}
			for _, edge := range edges {
				if !visited[edge.ToNodeID.String()] {
					nextQueue = append(nextQueue, edge.ToNodeID.String())
				}
			}
		}
		queue = nextQueue
	}
	return result, nil
}

func (s *GraphService) ShortestPath(ctx context.Context, fromNodeID, toNodeID string) ([]string, float64, error) {
	nodes, err := s.repo.FindNodesByGraph(ctx, "")
	if err != nil {
		return nil, 0, fmt.Errorf("shortest path: load nodes: %w", err)
	}

	dist := make(map[string]float64)
	prev := make(map[string]string)
	visited := make(map[string]bool)

	for _, n := range nodes {
		dist[n.ID.String()] = math.Inf(1)
	}
	dist[fromNodeID] = 0

	pq := &nodeQueue{}
	heap.Init(pq)
	heap.Push(pq, &pqItem{id: fromNodeID, dist: 0})

	for pq.Len() > 0 {
		current := heap.Pop(pq).(*pqItem)
		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		if current.id == toNodeID {
			break
		}

		edges, err := s.repo.FindOutgoingEdges(ctx, current.id)
		if err != nil {
			continue
		}
		for _, edge := range edges {
			toID := edge.ToNodeID.String()
			if visited[toID] {
				continue
			}
			newDist := dist[current.id] + edge.Weight
			if newDist < dist[toID] {
				dist[toID] = newDist
				prev[toID] = current.id
				heap.Push(pq, &pqItem{id: toID, dist: newDist})
			}
		}
	}

	if math.IsInf(dist[toNodeID], 1) {
		return nil, 0, fmt.Errorf("no path found from %s to %s", fromNodeID, toNodeID)
	}

	var path []string
	for at := toNodeID; at != ""; at = prev[at] {
		path = append([]string{at}, path...)
		if at == fromNodeID {
			break
		}
	}

	return path, dist[toNodeID], nil
}

type pqItem struct {
	id   string
	dist float64
	idx  int
}

type nodeQueue []*pqItem

func (q nodeQueue) Len() int           { return len(q) }
func (q nodeQueue) Less(i, j int) bool  { return q[i].dist < q[j].dist }
func (q nodeQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i]; q[i].idx = i; q[j].idx = j }
func (q *nodeQueue) Push(x interface{}) { item := x.(*pqItem); item.idx = len(*q); *q = append(*q, item) }
func (q *nodeQueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.idx = -1
	*q = old[0 : n-1]
	return item
}
