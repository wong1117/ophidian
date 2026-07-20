package workflow

import (
	"context"
	"sync"
	"time"
)

type WorkflowStatus string

const (
	StatusPending   WorkflowStatus = "PENDING"
	StatusRunning   WorkflowStatus = "RUNNING"
	StatusCompleted WorkflowStatus = "COMPLETED"
	StatusFailed    WorkflowStatus = "FAILED"
	StatusCancelled WorkflowStatus = "CANCELLED"
	StatusTimedOut  WorkflowStatus = "TIMED_OUT"
)

type NodeStatus string

const (
	NodePending   NodeStatus = "PENDING"
	NodeRunning   NodeStatus = "RUNNING"
	NodeCompleted NodeStatus = "COMPLETED"
	NodeFailed    NodeStatus = "FAILED"
	NodeSkipped   NodeStatus = "SKIPPED"
)

type Workflow struct {
	ID      string
	Name    string
	Nodes   []*Node
	Edges   []Edge
	Timeout time.Duration
}

type Node struct {
	ID         string
	Name       string
	MaxRetries int
	Timeout    time.Duration
	Metadata   map[string]interface{}
}

type Edge struct {
	From string
	To   string
}

type NodeExecutor interface {
	Execute(ctx context.Context, node *Node) error
}

type WorkflowEvent struct {
	WorkflowID string
	Type       WorkflowEventType
	NodeID     string
	Status     NodeStatus
	Error      string
	Timestamp  time.Time
}

type WorkflowEventType string

const (
	EventWorkflowStarted   WorkflowEventType = "WORKFLOW_STARTED"
	EventWorkflowCompleted WorkflowEventType = "WORKFLOW_COMPLETED"
	EventWorkflowFailed    WorkflowEventType = "WORKFLOW_FAILED"
	EventWorkflowCancelled WorkflowEventType = "WORKFLOW_CANCELLED"
	EventWorkflowTimedOut  WorkflowEventType = "WORKFLOW_TIMED_OUT"
	EventNodeStarted       WorkflowEventType = "NODE_STARTED"
	EventNodeCompleted     WorkflowEventType = "NODE_COMPLETED"
	EventNodeFailed        WorkflowEventType = "NODE_FAILED"
	EventNodeRetrying      WorkflowEventType = "NODE_RETRYING"
	EventNodeSkipped       WorkflowEventType = "NODE_SKIPPED"
)

type WorkflowResult struct {
	Status        WorkflowStatus
	NodeResults   map[string]NodeResult
	Events        []WorkflowEvent
	Duration      time.Duration
	CompletedNode int
	FailedNode    int
}

type NodeResult struct {
	Status     NodeStatus
	Error      string
	Attempts   int
	Duration   time.Duration
	StartedAt  time.Time
	CompletedAt time.Time
}

type WorkflowEngine struct {
	executor      NodeExecutor
	eventCh       chan WorkflowEvent
	statusTracker StatusTracker
	mu            sync.RWMutex
}

type StatusTracker interface {
	SaveStatus(ctx context.Context, workflowID string, status WorkflowStatus) error
	SaveNodeResult(ctx context.Context, workflowID, nodeID string, result NodeResult) error
}

func NewWorkflowEngine(executor NodeExecutor, tracker StatusTracker) *WorkflowEngine {
	return &WorkflowEngine{
		executor:      executor,
		eventCh:       make(chan WorkflowEvent, 100),
		statusTracker: tracker,
	}
}

func (e *WorkflowEngine) Events() <-chan WorkflowEvent {
	return e.eventCh
}

func (e *WorkflowEngine) Execute(ctx context.Context, wf *Workflow) *WorkflowResult {
	result := &WorkflowResult{
		NodeResults: make(map[string]NodeResult),
	}

	e.emit(WorkflowEvent{
		WorkflowID: wf.ID,
		Type:       EventWorkflowStarted,
		Timestamp:  time.Now(),
	})

	_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusRunning)

	if err := ValidateDAG(wf); err != nil {
		result.Status = StatusFailed
		e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowFailed, Error: err.Error(), Timestamp: time.Now()})
		_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusFailed)
		return result
	}

	execCtx, cancel := e.setupContext(ctx, wf)
	defer cancel()

	trigger := make(chan string, 1)
	startNodes := e.getRootNodes(wf)
	go func() {
		for _, id := range startNodes {
			trigger <- id
		}
	}()

	resultsCh := make(chan nodeExecution, len(wf.Nodes))
	var active int
	nodeMap := e.buildNodeMap(wf)

	startTime := time.Now()

	for active > 0 || len(resultsCh) > 0 || len(wf.Nodes) > 0 {
		select {
		case nodeID := <-trigger:
			active++
			go e.runNode(execCtx, wf.ID, nodeMap[nodeID], resultsCh, trigger)

		case exec := <-resultsCh:
			active--
			e.processNodeResult(wf, nodeMap, exec, result, trigger)

			if len(result.NodeResults) == len(wf.Nodes) {
				goto done
			}

		case <-execCtx.Done():
			if execCtx.Err() == context.DeadlineExceeded {
				result.Status = StatusTimedOut
				e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowTimedOut, Timestamp: time.Now()})
				_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusTimedOut)
			} else {
				result.Status = StatusCancelled
				e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowCancelled, Timestamp: time.Now()})
				_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusCancelled)
			}
			goto done
		}
	}

done:
	result.Duration = time.Since(startTime)
	result.CompletedNode = countByStatus(result, NodeCompleted)
	result.FailedNode = countByStatus(result, NodeFailed)

	if result.Status == "" {
		if result.FailedNode > 0 {
			result.Status = StatusFailed
			e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowFailed, Timestamp: time.Now()})
			_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusFailed)
		} else {
			result.Status = StatusCompleted
			e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowCompleted, Timestamp: time.Now()})
			_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusCompleted)
		}
	}

	return result
}

func (e *WorkflowEngine) setupContext(ctx context.Context, wf *Workflow) (context.Context, context.CancelFunc) {
	if wf.Timeout > 0 {
		return context.WithTimeout(ctx, wf.Timeout)
	}
	return context.WithCancel(ctx)
}

func (e *WorkflowEngine) runNode(ctx context.Context, workflowID string, node *Node, resultsCh chan<- nodeExecution, trigger chan<- string) {
	result := nodeExecution{nodeID: node.ID, startedAt: time.Now()}
	var execErr error

	for attempt := 1; attempt <= node.MaxRetries+1; attempt++ {
		nodeCtx := ctx
		if node.Timeout > 0 {
			var cancel context.CancelFunc
			nodeCtx, cancel = context.WithTimeout(ctx, node.Timeout)
			defer cancel()
		}

		e.emit(WorkflowEvent{WorkflowID: workflowID, NodeID: node.ID, Type: EventNodeStarted, Timestamp: time.Now()})

		execErr = e.executor.Execute(nodeCtx, node)
		result.attempts = attempt

		if execErr == nil {
			e.emit(WorkflowEvent{WorkflowID: workflowID, NodeID: node.ID, Type: EventNodeCompleted, Timestamp: time.Now()})
			break
		}

		if attempt <= node.MaxRetries {
			e.emit(WorkflowEvent{WorkflowID: workflowID, NodeID: node.ID, Type: EventNodeRetrying, Error: execErr.Error(), Timestamp: time.Now()})
		}
	}

	result.err = execErr
	result.duration = time.Since(result.startedAt)
	resultsCh <- result
}

func (e *WorkflowEngine) processNodeResult(
	wf *Workflow,
	_ map[string]*Node,
	exec nodeExecution,
	result *WorkflowResult,
	trigger chan<- string,
) {
	nr := NodeResult{
		Attempts:    exec.attempts,
		Duration:    exec.duration,
		StartedAt:   exec.startedAt,
		CompletedAt: time.Now(),
	}

	if exec.err != nil {
		nr.Status = NodeFailed
		nr.Error = exec.err.Error()
		e.emit(WorkflowEvent{WorkflowID: wf.ID, NodeID: exec.nodeID, Type: EventNodeFailed, Error: exec.err.Error(), Timestamp: time.Now()})

		for _, edge := range wf.Edges {
			if edge.From == exec.nodeID {
				depID := edge.To
				if _, exists := result.NodeResults[depID]; !exists {
					nr := NodeResult{Status: NodeSkipped}
					result.NodeResults[depID] = nr
					e.emit(WorkflowEvent{WorkflowID: wf.ID, NodeID: depID, Type: EventNodeSkipped, Timestamp: time.Now()})
					_ = e.statusTracker.SaveNodeResult(context.Background(), wf.ID, depID, nr)
				}
			}
		}
	} else {
		nr.Status = NodeCompleted
	}

	result.NodeResults[exec.nodeID] = nr
	_ = e.statusTracker.SaveNodeResult(context.Background(), wf.ID, exec.nodeID, nr)

	for _, edge := range wf.Edges {
		if edge.From == exec.nodeID {
			depID := edge.To
			depsReady := true
			for _, e2 := range wf.Edges {
				if e2.To == depID {
					if r, ok := result.NodeResults[e2.From]; !ok || r.Status != NodeCompleted {
						depsReady = false
						break
					}
				}
			}
			if depsReady {
				if _, processed := result.NodeResults[depID]; !processed {
					trigger <- depID
				}
			}
		}
	}
}

func (e *WorkflowEngine) getRootNodes(wf *Workflow) []string {
	hasDependency := make(map[string]bool)
	for _, edge := range wf.Edges {
		hasDependency[edge.To] = true
	}
	var roots []string
	for _, node := range wf.Nodes {
		if !hasDependency[node.ID] {
			roots = append(roots, node.ID)
		}
	}
	return roots
}

func (e *WorkflowEngine) buildNodeMap(wf *Workflow) map[string]*Node {
	m := make(map[string]*Node)
	for _, node := range wf.Nodes {
		m[node.ID] = node
	}
	return m
}

func (e *WorkflowEngine) emit(event WorkflowEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	select {
	case e.eventCh <- event:
	default:
	}
}

func countByStatus(result *WorkflowResult, status NodeStatus) int {
	count := 0
	for _, r := range result.NodeResults {
		if r.Status == status {
			count++
		}
	}
	return count
}

type nodeExecution struct {
	nodeID    string
	err       error
	attempts  int
	duration  time.Duration
	startedAt time.Time
}
