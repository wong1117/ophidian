package graph

import (
	domainGraph "github.com/ophidian/ophidian/internal/domain/graph"
)

type TestGraphRepo struct {
	Nodes map[string]domainGraph.Node
	Edges []domainGraph.Edge
}

func NewTestGraphRepo() *TestGraphRepo {
	return &TestGraphRepo{
		Nodes: make(map[string]domainGraph.Node),
		Edges: make([]domainGraph.Edge, 0),
	}
}

func (r *TestGraphRepo) AddNode(n domainGraph.Node)        { r.Nodes[n.ID.String()] = n }
func (r *TestGraphRepo) AddEdge(e domainGraph.Edge)         { r.Edges = append(r.Edges, e) }

func (r *TestGraphRepo) SaveGraph(ctx interface{}, g *domainGraph.Graph) error  { return nil }
func (r *TestGraphRepo) FindGraphByID(ctx interface{}, id string) (*domainGraph.Graph, error) { return nil, nil }
func (r *TestGraphRepo) FindGraphsByName(ctx interface{}, name string) ([]*domainGraph.Graph, error) { return nil, nil }
func (r *TestGraphRepo) SaveNode(ctx interface{}, n *domainGraph.Node) error    { return nil }
func (r *TestGraphRepo) FindNodeByID(ctx interface{}, id string) (*domainGraph.Node, error) {
	if n, ok := r.Nodes[id]; ok { return &n, nil }
	return nil, nil
}
func (r *TestGraphRepo) FindNodesByGraph(ctx interface{}, graphID string) ([]*domainGraph.Node, error) {
	var result []*domainGraph.Node
	for _, n := range r.Nodes { cp := n; result = append(result, &cp) }
	return result, nil
}
func (r *TestGraphRepo) FindNodesByEntity(ctx interface{}, et, eid string) ([]*domainGraph.Node, error) { return nil, nil }
func (r *TestGraphRepo) DeleteNode(ctx interface{}, id string) error              { return nil }
func (r *TestGraphRepo) SaveEdge(ctx interface{}, e *domainGraph.Edge) error      { return nil }
func (r *TestGraphRepo) FindEdgeByID(ctx interface{}, id string) (*domainGraph.Edge, error) { return nil, nil }
func (r *TestGraphRepo) FindEdgesByGraph(ctx interface{}, graphID string) ([]*domainGraph.Edge, error) {
	result := make([]*domainGraph.Edge, len(r.Edges))
	for i := range r.Edges { result[i] = &r.Edges[i] }
	return result, nil
}
func (r *TestGraphRepo) FindOutgoingEdges(ctx interface{}, nodeID string) ([]*domainGraph.Edge, error) {
	var result []*domainGraph.Edge
	for i := range r.Edges {
		if r.Edges[i].FromNodeID.String() == nodeID { cp := r.Edges[i]; result = append(result, &cp) }
	}
	return result, nil
}
func (r *TestGraphRepo) FindIncomingEdges(ctx interface{}, nodeID string) ([]*domainGraph.Edge, error) { return nil, nil }
func (r *TestGraphRepo) DeleteEdge(ctx interface{}, id string) error              { return nil }
