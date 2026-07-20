package graph

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainGraph "github.com/ophidian/ophidian/internal/domain/graph"
	"github.com/stretchr/testify/assert"
)

type testGraphRepo struct {
	nodes map[string]*domainGraph.Node
	edges map[string]*domainGraph.Edge
}

func newTestGraphRepo() *testGraphRepo {
	return &testGraphRepo{
		nodes: make(map[string]*domainGraph.Node),
		edges: make(map[string]*domainGraph.Edge),
	}
}

func (r *testGraphRepo) SaveGraph(ctx context.Context, g *domainGraph.Graph) error { return nil }
func (r *testGraphRepo) FindGraphByID(ctx context.Context, id string) (*domainGraph.Graph, error) {
	return nil, nil
}
func (r *testGraphRepo) FindGraphsByName(ctx context.Context, name string) ([]*domainGraph.Graph, error) {
	return nil, nil
}
func (r *testGraphRepo) SaveNode(ctx context.Context, n *domainGraph.Node) error {
	r.nodes[n.ID.String()] = n
	return nil
}
func (r *testGraphRepo) FindNodeByID(ctx context.Context, id string) (*domainGraph.Node, error) {
	n, ok := r.nodes[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return n, nil
}
func (r *testGraphRepo) FindNodesByGraph(ctx context.Context, graphID string) ([]*domainGraph.Node, error) {
	var result []*domainGraph.Node
	for _, n := range r.nodes {
		result = append(result, n)
	}
	return result, nil
}
func (r *testGraphRepo) FindNodesByEntity(ctx context.Context, entityType, entityID string) ([]*domainGraph.Node, error) {
	var result []*domainGraph.Node
	for _, n := range r.nodes {
		if n.EntityType == entityType && n.EntityID.String() == entityID {
			result = append(result, n)
		}
	}
	return result, nil
}
func (r *testGraphRepo) DeleteNode(ctx context.Context, id string) error {
	delete(r.nodes, id)
	return nil
}
func (r *testGraphRepo) SaveEdge(ctx context.Context, e *domainGraph.Edge) error {
	r.edges[e.ID.String()] = e
	return nil
}
func (r *testGraphRepo) FindEdgeByID(ctx context.Context, id string) (*domainGraph.Edge, error) {
	e, ok := r.edges[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return e, nil
}
func (r *testGraphRepo) FindEdgesByGraph(ctx context.Context, graphID string) ([]*domainGraph.Edge, error) {
	var result []*domainGraph.Edge
	for _, e := range r.edges {
		result = append(result, e)
	}
	return result, nil
}
func (r *testGraphRepo) FindOutgoingEdges(ctx context.Context, nodeID string) ([]*domainGraph.Edge, error) {
	var result []*domainGraph.Edge
	for _, e := range r.edges {
		if e.FromNodeID.String() == nodeID {
			result = append(result, e)
		}
	}
	return result, nil
}
func (r *testGraphRepo) FindIncomingEdges(ctx context.Context, nodeID string) ([]*domainGraph.Edge, error) {
	var result []*domainGraph.Edge
	for _, e := range r.edges {
		if e.ToNodeID.String() == nodeID {
			result = append(result, e)
		}
	}
	return result, nil
}
func (r *testGraphRepo) DeleteEdge(ctx context.Context, id string) error {
	delete(r.edges, id)
	return nil
}

func TestGraphService_AddNode(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	node, err := svc.AddNode(context.Background(), "g1", "mission", "m1", []string{"active"}, map[string]interface{}{"name": "test"})

	assert.NoError(t, err)
	assert.Equal(t, "mission", node.EntityType)
	assert.Equal(t, common.ID("m1"), node.EntityID)
	assert.Contains(t, node.Labels, "active")
}

func TestGraphService_AddEdge(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	_, _ = svc.AddNode(context.Background(), "g1", "host", "h1", nil, nil)
	_, _ = svc.AddNode(context.Background(), "g1", "service", "s1", nil, nil)

	edge, err := svc.AddEdge(context.Background(), "g1", "h1", "s1", "RUNS", 0.5, map[string]interface{}{"port": "8080"})

	assert.NoError(t, err)
	assert.Equal(t, "RUNS", edge.Type)
	assert.Equal(t, 0.5, edge.Weight)
}

func TestGraphService_Traverse(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	na, _ := svc.AddNode(context.Background(), "g1", "a", "a", nil, nil)
	nb, _ := svc.AddNode(context.Background(), "g1", "b", "b", nil, nil)
	nc, _ := svc.AddNode(context.Background(), "g1", "c", "c", nil, nil)
	svc.AddEdge(context.Background(), "g1", na.ID.String(), nb.ID.String(), "TO", 1, nil)
	svc.AddEdge(context.Background(), "g1", nb.ID.String(), nc.ID.String(), "TO", 1, nil)

	result, err := svc.Traverse(context.Background(), na.ID.String(), 3)

	assert.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestGraphService_ShortestPath(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	na := &domainGraph.Node{ID: common.ID("a"), EntityType: "a"}
	nb := &domainGraph.Node{ID: common.ID("b"), EntityType: "b"}
	nc := &domainGraph.Node{ID: common.ID("c"), EntityType: "c"}
	repo.nodes["a"] = na
	repo.nodes["b"] = nb
	repo.nodes["c"] = nc

	svc.AddEdge(context.Background(), "g1", "a", "b", "TO", 2, nil)
	svc.AddEdge(context.Background(), "g1", "a", "c", "TO", 5, nil)
	svc.AddEdge(context.Background(), "g1", "b", "c", "TO", 1, nil)

	path, dist, err := svc.ShortestPath(context.Background(), "a", "c")

	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, path)
	assert.Equal(t, 3.0, dist)
}

func TestGraphService_ShortestPath_NoPath(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	repo.nodes["x"] = &domainGraph.Node{ID: "x", EntityType: "x"}
	repo.nodes["y"] = &domainGraph.Node{ID: "y", EntityType: "y"}

	_, _, err := svc.ShortestPath(context.Background(), "x", "y")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no path found")
}

func TestGraphService_DeleteNode(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	svc.AddNode(context.Background(), "g1", "a", "a", nil, nil)
	n2, _ := svc.AddNode(context.Background(), "g1", "b", "b", nil, nil)
	svc.AddEdge(context.Background(), "g1", "a", "b", "TO", 1, nil)

	err := svc.DeleteNode(context.Background(), n2.ID.String())
	assert.NoError(t, err)

	_, err = repo.FindNodeByID(context.Background(), n2.ID.String())
	assert.Error(t, err)

	edges, _ := repo.FindOutgoingEdges(context.Background(), n2.ID.String())
	assert.Empty(t, edges)
}

func TestGraphService_DeleteEdge(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	svc.AddNode(context.Background(), "g1", "a", "a", nil, nil)
	svc.AddNode(context.Background(), "g1", "b", "b", nil, nil)
	e, _ := svc.AddEdge(context.Background(), "g1", "a", "b", "TO", 1, nil)

	err := svc.DeleteEdge(context.Background(), e.ID.String())
	assert.NoError(t, err)

	_, err = repo.FindEdgeByID(context.Background(), e.ID.String())
	assert.Error(t, err)
}

func TestGraphService_GetNodeByEntity(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	svc.AddNode(context.Background(), "g1", "mission", "m123", nil, nil)

	node, err := svc.GetNodeByEntity(context.Background(), "mission", "m123")
	assert.NoError(t, err)
	assert.Equal(t, common.ID("m123"), node.EntityID)
}

func TestGraphService_TraverseDepth3(t *testing.T) {
	repo := newTestGraphRepo()
	svc := NewGraphService(repo)

	ids := []string{}
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("n%d", i)
		ids = append(ids, id)
		repo.nodes[id] = &domainGraph.Node{ID: common.ID(id), EntityType: fmt.Sprintf("type%d", i)}
	}
	for i := 0; i < 9; i++ {
		svc.AddEdge(context.Background(), "g1", ids[i], ids[i+1], "NEXT", 1, nil)
	}

	result, err := svc.Traverse(context.Background(), "n0", 3)
	assert.NoError(t, err)
	assert.Len(t, result, 3)

	sort.Slice(result, func(i, j int) bool { return result[i].ID.String() < result[j].ID.String() })
}
