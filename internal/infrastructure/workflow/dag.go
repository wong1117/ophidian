package workflow

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

var (
	ErrInvalidDAG        = errors.New("invalid dag")
	ErrNodeNotFound      = errors.New("node not found")
	ErrCyclicDependency  = errors.New("cyclic dependency detected")
	ErrDuplicateNode     = errors.New("duplicate node id")
	ErrEmptyWorkflow     = errors.New("workflow has no nodes")
	ErrWorkflowFailed    = errors.New("workflow execution failed")
	ErrWorkflowTimeout   = errors.New("workflow timed out")
	ErrWorkflowCancelled = errors.New("workflow cancelled")
	ErrNodeTimeout       = errors.New("node execution timed out")
	ErrConcurrencyLimit  = errors.New("concurrency limit reached")
)

func ValidateDAG(wf *Workflow) error {
	if len(wf.Nodes) == 0 {
		return fmt.Errorf("%w: %w", ErrInvalidDAG, ErrEmptyWorkflow)
	}

	nodeIDs := make(map[string]bool)
	for _, node := range wf.Nodes {
		if node.ID == "" {
			return fmt.Errorf("%w: node has empty id", ErrInvalidDAG)
		}
		if nodeIDs[node.ID] {
			return fmt.Errorf("%w: %w: %s", ErrInvalidDAG, ErrDuplicateNode, node.ID)
		}
		nodeIDs[node.ID] = true
	}

	for _, edge := range wf.Edges {
		if !nodeIDs[edge.From] {
			return fmt.Errorf("%w: %w: from node %s not found", ErrInvalidDAG, ErrNodeNotFound, edge.From)
		}
		if !nodeIDs[edge.To] {
			return fmt.Errorf("%w: %w: to node %s not found", ErrInvalidDAG, ErrNodeNotFound, edge.To)
		}
		if edge.From == edge.To {
			return fmt.Errorf("%w: self-referencing edge detected for node %s", ErrInvalidDAG, edge.From)
		}
	}

	if hasCycle(nodeIDs, wf.Edges) {
		return fmt.Errorf("%w: %w", ErrInvalidDAG, ErrCyclicDependency)
	}

	return nil
}

func hasCycle(nodeIDs map[string]bool, edges []Edge) bool {
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	for id := range nodeIDs {
		inDegree[id] = 0
	}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		inDegree[e.To]++
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, neighbor := range adj[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	return visited != len(nodeIDs)
}

type BackoffFunc func(attempt int) time.Duration

func DefaultBackoff() BackoffFunc {
	return ExponentialBackoff(100*time.Millisecond, 30*time.Second, 0.1)
}

func ExponentialBackoff(base, max time.Duration, jitter float64) BackoffFunc {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(attempt int) time.Duration {
		if attempt <= 1 {
			return 0
		}
		backoff := float64(base) * math.Pow(2, float64(attempt-1))
		if backoff > float64(max) {
			backoff = float64(max)
		}
		jitterAmount := backoff * jitter
		backoff += (rng.Float64()*2 - 1) * jitterAmount
		return time.Duration(backoff)
	}
}
