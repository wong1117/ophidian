package graph

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domainGraph "github.com/ophidian/ophidian/internal/domain/graph"

	postgres "github.com/ophidian/ophidian/internal/infrastructure/persistence/postgres"
)

type GraphRepo struct {
	deps postgres.RepoDeps
}

func NewGraphRepo(pool *pgxpool.Pool) *GraphRepo {
	return &GraphRepo{deps: postgres.RepoDepsFromPool(pool)}
}

func NewGraphRepoWithDeps(deps postgres.RepoDeps) *GraphRepo {
	return &GraphRepo{deps: deps}
}

func (r *GraphRepo) SaveGraph(ctx context.Context, g *domainGraph.Graph) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO knowledge_graphs (id, name, description, version, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		g.ID, g.Name, g.Description, g.Version, postgres.MarshalJSON(g.Metadata),
		g.CreatedAt, g.UpdatedAt,
	)
	return fmt.Errorf("save graph: %w", err)
}

func (r *GraphRepo) FindGraphByID(ctx context.Context, id string) (*domainGraph.Graph, error) {
	var g domainGraph.Graph
	var metaJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, name, description, version, metadata, created_at, updated_at
		 FROM knowledge_graphs WHERE id = $1`, id,
	).Scan(&g.ID, &g.Name, &g.Description, &g.Version, &metaJSON, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("graph not found: %s", id)
		}
		return nil, fmt.Errorf("find graph: %w", err)
	}
	if len(metaJSON) > 0 && string(metaJSON) != "null" {
		postgres.UnmarshalJSON(metaJSON, &g.Metadata)
	}
	return &g, nil
}

func (r *GraphRepo) FindGraphsByName(ctx context.Context, name string) ([]*domainGraph.Graph, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, name, description, version, metadata, created_at, updated_at
		 FROM knowledge_graphs WHERE name = $1 ORDER BY version DESC`, name)
	if err != nil {
		return nil, fmt.Errorf("find graphs by name: %w", err)
	}
	defer rows.Close()
	var graphs []*domainGraph.Graph
	for rows.Next() {
		var g domainGraph.Graph
		var metaJSON []byte
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Version, &metaJSON, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("find graphs scan: %w", err)
		}
		if len(metaJSON) > 0 && string(metaJSON) != "null" {
			postgres.UnmarshalJSON(metaJSON, &g.Metadata)
		}
		graphs = append(graphs, &g)
	}
	return graphs, rows.Err()
}

func (r *GraphRepo) SaveNode(ctx context.Context, n *domainGraph.Node) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO knowledge_graph_nodes (id, entity_type, entity_id, labels, properties, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		n.ID, n.EntityType, n.EntityID,
		postgres.MarshalJSON(n.Labels), postgres.MarshalJSON(n.Properties),
		n.CreatedAt, n.UpdatedAt,
	)
	return fmt.Errorf("save node: %w", err)
}

func (r *GraphRepo) FindNodeByID(ctx context.Context, id string) (*domainGraph.Node, error) {
	var n domainGraph.Node
	var labelsJSON, propsJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, entity_type, entity_id, labels, properties, created_at, updated_at
		 FROM knowledge_graph_nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.EntityType, &n.EntityID, &labelsJSON, &propsJSON, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("node not found: %s", id)
		}
		return nil, fmt.Errorf("find node: %w", err)
	}
	postgres.UnmarshalJSON(labelsJSON, &n.Labels)
	postgres.UnmarshalJSON(propsJSON, &n.Properties)
	return &n, nil
}

func (r *GraphRepo) FindNodesByGraph(ctx context.Context, graphID string) ([]*domainGraph.Node, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, entity_type, entity_id, labels, properties, created_at, updated_at
		 FROM knowledge_graph_nodes ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("find nodes by graph: %w", err)
	}
	defer rows.Close()
	var nodes []*domainGraph.Node
	for rows.Next() {
		var n domainGraph.Node
		var labelsJSON, propsJSON []byte
		if err := rows.Scan(&n.ID, &n.EntityType, &n.EntityID, &labelsJSON, &propsJSON, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("find nodes scan: %w", err)
		}
		postgres.UnmarshalJSON(labelsJSON, &n.Labels)
		postgres.UnmarshalJSON(propsJSON, &n.Properties)
		nodes = append(nodes, &n)
	}
	return nodes, rows.Err()
}

func (r *GraphRepo) FindNodesByEntity(ctx context.Context, entityType, entityID string) ([]*domainGraph.Node, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, entity_type, entity_id, labels, properties, created_at, updated_at
		 FROM knowledge_graph_nodes WHERE entity_type = $1 AND entity_id = $2`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("find nodes by entity: %w", err)
	}
	defer rows.Close()
	var nodes []*domainGraph.Node
	for rows.Next() {
		var n domainGraph.Node
		var labelsJSON, propsJSON []byte
		if err := rows.Scan(&n.ID, &n.EntityType, &n.EntityID, &labelsJSON, &propsJSON, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("find nodes by entity scan: %w", err)
		}
		postgres.UnmarshalJSON(labelsJSON, &n.Labels)
		postgres.UnmarshalJSON(propsJSON, &n.Properties)
		nodes = append(nodes, &n)
	}
	return nodes, rows.Err()
}

func (r *GraphRepo) DeleteNode(ctx context.Context, id string) error {
	_, err := r.deps.Exec(ctx, `DELETE FROM knowledge_graph_nodes WHERE id = $1`, id)
	return fmt.Errorf("delete node: %w", err)
}

func (r *GraphRepo) SaveEdge(ctx context.Context, e *domainGraph.Edge) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO knowledge_graph_edges (id, graph_id, from_node_id, to_node_id, type, weight, properties, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ID, e.GraphID, e.FromNodeID, e.ToNodeID, e.Type, e.Weight,
		postgres.MarshalJSON(e.Properties), e.CreatedAt,
	)
	return fmt.Errorf("save edge: %w", err)
}

func (r *GraphRepo) FindEdgeByID(ctx context.Context, id string) (*domainGraph.Edge, error) {
	var e domainGraph.Edge
	var propsJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, graph_id, from_node_id, to_node_id, type, weight, properties, created_at
		 FROM knowledge_graph_edges WHERE id = $1`, id,
	).Scan(&e.ID, &e.GraphID, &e.FromNodeID, &e.ToNodeID, &e.Type, &e.Weight, &propsJSON, &e.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("edge not found: %s", id)
		}
		return nil, fmt.Errorf("find edge: %w", err)
	}
	postgres.UnmarshalJSON(propsJSON, &e.Properties)
	return &e, nil
}

func (r *GraphRepo) FindEdgesByGraph(ctx context.Context, graphID string) ([]*domainGraph.Edge, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, graph_id, from_node_id, to_node_id, type, weight, properties, created_at
		 FROM knowledge_graph_edges WHERE graph_id = $1`, graphID)
	if err != nil {
		return nil, fmt.Errorf("find edges by graph: %w", err)
	}
	defer rows.Close()
	var edges []*domainGraph.Edge
	for rows.Next() {
		var e domainGraph.Edge
		var propsJSON []byte
		if err := rows.Scan(&e.ID, &e.GraphID, &e.FromNodeID, &e.ToNodeID, &e.Type, &e.Weight, &propsJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("find edges scan: %w", err)
		}
		postgres.UnmarshalJSON(propsJSON, &e.Properties)
		edges = append(edges, &e)
	}
	return edges, rows.Err()
}

func (r *GraphRepo) FindOutgoingEdges(ctx context.Context, nodeID string) ([]*domainGraph.Edge, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, graph_id, from_node_id, to_node_id, type, weight, properties, created_at
		 FROM knowledge_graph_edges WHERE from_node_id = $1`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("find outgoing edges: %w", err)
	}
	defer rows.Close()
	var edges []*domainGraph.Edge
	for rows.Next() {
		var e domainGraph.Edge
		var propsJSON []byte
		if err := rows.Scan(&e.ID, &e.GraphID, &e.FromNodeID, &e.ToNodeID, &e.Type, &e.Weight, &propsJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("find outgoing edges scan: %w", err)
		}
		postgres.UnmarshalJSON(propsJSON, &e.Properties)
		edges = append(edges, &e)
	}
	return edges, rows.Err()
}

func (r *GraphRepo) FindIncomingEdges(ctx context.Context, nodeID string) ([]*domainGraph.Edge, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, graph_id, from_node_id, to_node_id, type, weight, properties, created_at
		 FROM knowledge_graph_edges WHERE to_node_id = $1`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("find incoming edges: %w", err)
	}
	defer rows.Close()
	var edges []*domainGraph.Edge
	for rows.Next() {
		var e domainGraph.Edge
		var propsJSON []byte
		if err := rows.Scan(&e.ID, &e.GraphID, &e.FromNodeID, &e.ToNodeID, &e.Type, &e.Weight, &propsJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("find incoming edges scan: %w", err)
		}
		postgres.UnmarshalJSON(propsJSON, &e.Properties)
		edges = append(edges, &e)
	}
	return edges, rows.Err()
}

func (r *GraphRepo) DeleteEdge(ctx context.Context, id string) error {
	_, err := r.deps.Exec(ctx, `DELETE FROM knowledge_graph_edges WHERE id = $1`, id)
	return fmt.Errorf("delete edge: %w", err)
}
