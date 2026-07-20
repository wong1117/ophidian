package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockExecutor struct {
	mu       sync.Mutex
	execFn   func(ctx context.Context, node *Node) error
	executed map[string]int
	delays   map[string]time.Duration
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		executed: make(map[string]int),
		delays:   make(map[string]time.Duration),
	}
}

func (m *mockExecutor) Execute(ctx context.Context, node *Node) error {
	m.mu.Lock()
	m.executed[node.ID]++
	m.mu.Unlock()

	if m.execFn != nil {
		return m.execFn(ctx, node)
	}

	if d, ok := m.delays[node.ID]; ok {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

type mockStatusTracker struct {
	statuses    map[string]WorkflowStatus
	nodeResults map[string]map[string]NodeResult
}

func newMockStatusTracker() *mockStatusTracker {
	return &mockStatusTracker{
		statuses:    make(map[string]WorkflowStatus),
		nodeResults: make(map[string]map[string]NodeResult),
	}
}

func (m *mockStatusTracker) SaveStatus(ctx context.Context, id string, status WorkflowStatus) error {
	m.statuses[id] = status
	return nil
}

func (m *mockStatusTracker) SaveNodeResult(ctx context.Context, wfID, nodeID string, result NodeResult) error {
	if m.nodeResults[wfID] == nil {
		m.nodeResults[wfID] = make(map[string]NodeResult)
	}
	m.nodeResults[wfID][nodeID] = result
	return nil
}

func linearWorkflow() *Workflow {
	return &Workflow{
		ID:   "wf-linear",
		Name: "Linear Test",
		Nodes: []*Node{
			{ID: "a", Name: "Node A"},
			{ID: "b", Name: "Node B"},
			{ID: "c", Name: "Node C"},
		},
		Edges: []Edge{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
		},
	}
}

func dagWorkflow() *Workflow {
	return &Workflow{
		ID:   "wf-dag",
		Name: "DAG Test",
		Nodes: []*Node{
			{ID: "a", Name: "Node A"},
			{ID: "b", Name: "Node B"},
			{ID: "c", Name: "Node C"},
			{ID: "d", Name: "Node D"},
		},
		Edges: []Edge{
			{From: "a", To: "c"},
			{From: "b", To: "c"},
			{From: "c", To: "d"},
		},
	}
}

func TestWorkflowEngine_LinearSuccess(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := linearWorkflow()
	ctx := context.Background()

	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusCompleted, result.Status)
	assert.Len(t, result.NodeResults, 3)
	assert.Equal(t, NodeCompleted, result.NodeResults["a"].Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["b"].Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["c"].Status)
	assert.Equal(t, 3, result.CompletedNode)
	assert.Equal(t, 0, result.FailedNode)
	assert.NotZero(t, result.Duration)

	assert.Equal(t, 1, exec.executed["a"])
	assert.Equal(t, 1, exec.executed["b"])
	assert.Equal(t, 1, exec.executed["c"])

	events := drainEvents(engine)
	assert.NotEmpty(t, events)

	assert.Equal(t, StatusCompleted, tracker.statuses["wf-linear"])
}

func TestWorkflowEngine_DAGParallel(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	record := make(map[string]time.Time)
	var mu sync.Mutex
	exec.execFn = func(ctx context.Context, node *Node) error {
		mu.Lock()
		record[node.ID] = time.Now()
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	wf := dagWorkflow()
	ctx := context.Background()

	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusCompleted, result.Status)
	assert.Len(t, result.NodeResults, 4)

	assert.Equal(t, NodeCompleted, result.NodeResults["a"].Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["b"].Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["c"].Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["d"].Status)

	assert.True(t, record["c"].After(record["a"]))
	assert.True(t, record["c"].After(record["b"]))
	assert.True(t, record["d"].After(record["c"]))
}

func TestWorkflowEngine_RetrySuccess(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID: "wf-retry",
		Nodes: []*Node{
			{ID: "a", Name: "Node A", MaxRetries: 2},
		},
	}

	attempts := 0
	exec.execFn = func(ctx context.Context, node *Node) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	ctx := context.Background()
	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["a"].Status)
	assert.Equal(t, 3, result.NodeResults["a"].Attempts)
	assert.Equal(t, 3, attempts)

	events := drainEvents(engine)
	retryCount := 0
	for _, ev := range events {
		if ev.Type == EventNodeRetrying {
			retryCount++
		}
	}
	assert.Equal(t, 2, retryCount)
}

func TestWorkflowEngine_RetryExhausted(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID: "wf-retry-fail",
		Nodes: []*Node{
			{ID: "a", Name: "Node A", MaxRetries: 1},
		},
	}

	exec.execFn = func(ctx context.Context, node *Node) error {
		return errors.New("permanent error")
	}

	ctx := context.Background()
	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, NodeFailed, result.NodeResults["a"].Status)
	assert.Equal(t, 2, result.NodeResults["a"].Attempts)
	assert.Equal(t, 1, result.FailedNode)
	assert.Contains(t, result.NodeResults["a"].Error, "permanent error")
}

func TestWorkflowEngine_NodeFailureSkipsDependents(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := linearWorkflow()
	exec.execFn = func(ctx context.Context, node *Node) error {
		if node.ID == "b" {
			return errors.New("node b failed")
		}
		return nil
	}

	ctx := context.Background()
	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["a"].Status)
	assert.Equal(t, NodeFailed, result.NodeResults["b"].Status)
	assert.Equal(t, NodeSkipped, result.NodeResults["c"].Status)

	events := drainEvents(engine)
	assert.NotEmpty(t, events)
	assertHasEvent(t, events, EventNodeSkipped, "c")
}

func TestWorkflowEngine_Timeout(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID:      "wf-timeout",
		Name:    "Timeout Test",
		Timeout: 50 * time.Millisecond,
		Nodes: []*Node{
			{ID: "a", Name: "Node A"},
		},
	}

	exec.delays["a"] = 200 * time.Millisecond

	ctx := context.Background()
	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusTimedOut, result.Status)

	events := drainEvents(engine)
	assertHasEvent(t, events, EventWorkflowTimedOut, "")
}

func TestWorkflowEngine_Cancellation(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID: "wf-cancel",
		Nodes: []*Node{
			{ID: "a", Name: "Node A"},
			{ID: "b", Name: "Node B"},
		},
		Edges: []Edge{{From: "a", To: "b"}},
	}

	exec.delays["a"] = 500 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusCancelled, result.Status)

	events := drainEvents(engine)
	assertHasEvent(t, events, EventWorkflowCancelled, "")
}

func TestWorkflowEngine_NodeTimeout(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID: "wf-node-timeout",
		Nodes: []*Node{
			{ID: "a", Name: "Node A", Timeout: 30 * time.Millisecond},
		},
	}

	exec.delays["a"] = 200 * time.Millisecond

	ctx := context.Background()
	result := engine.Execute(ctx, wf)

	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, NodeFailed, result.NodeResults["a"].Status)
	assert.Contains(t, result.NodeResults["a"].Error, "deadline exceeded")
}

func TestWorkflowEngine_EmptyWorkflow(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	result := engine.Execute(context.Background(), &Workflow{ID: "empty"})

	assert.Equal(t, StatusFailed, result.Status)
	assert.Len(t, result.NodeResults, 0)
}

func TestValidateDAG_ValidLinear(t *testing.T) {
	err := ValidateDAG(linearWorkflow())
	assert.NoError(t, err)
}

func TestValidateDAG_ValidDAG(t *testing.T) {
	err := ValidateDAG(dagWorkflow())
	assert.NoError(t, err)
}

func TestValidateDAG_EmptyNodes(t *testing.T) {
	err := ValidateDAG(&Workflow{ID: "empty"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDAG)
	assert.ErrorIs(t, err, ErrEmptyWorkflow)
}

func TestValidateDAG_EmptyNodeID(t *testing.T) {
	wf := &Workflow{
		ID:    "test",
		Nodes: []*Node{{ID: "", Name: "bad"}},
	}
	err := ValidateDAG(wf)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDAG)
}

func TestValidateDAG_DuplicateNode(t *testing.T) {
	wf := &Workflow{
		ID: "test",
		Nodes: []*Node{
			{ID: "a", Name: "A"},
			{ID: "a", Name: "A Dup"},
		},
	}
	err := ValidateDAG(wf)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDuplicateNode)
}

func TestValidateDAG_MissingEdgeNode(t *testing.T) {
	wf := &Workflow{
		ID:    "test",
		Nodes: []*Node{{ID: "a"}},
		Edges: []Edge{{From: "a", To: "b"}},
	}
	err := ValidateDAG(wf)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNodeNotFound)
}

func TestValidateDAG_SelfReference(t *testing.T) {
	wf := &Workflow{
		ID:    "test",
		Nodes: []*Node{{ID: "a"}},
		Edges: []Edge{{From: "a", To: "a"}},
	}
	err := ValidateDAG(wf)
	assert.Error(t, err)
}

func TestValidateDAG_Cycle(t *testing.T) {
	wf := &Workflow{
		ID: "test",
		Nodes: []*Node{
			{ID: "a"}, {ID: "b"}, {ID: "c"},
		},
		Edges: []Edge{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "c", To: "a"},
		},
	}
	err := ValidateDAG(wf)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCyclicDependency)
}

func TestWorkflowEngine_EventsChannel(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID: "wf-events",
		Nodes: []*Node{
			{ID: "a", Name: "Node A"},
		},
	}

	var mu sync.Mutex
	var collected []WorkflowEvent

	go func() {
		for {
			select {
			case ev := <-engine.Events():
				mu.Lock()
				collected = append(collected, ev)
				mu.Unlock()
			case <-time.After(200 * time.Millisecond):
				return
			}
		}
	}()

	result := engine.Execute(context.Background(), wf)
	assert.Equal(t, StatusCompleted, result.Status)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, len(collected), 3)
	assertHasEvent(t, collected, EventWorkflowStarted, "")
	assertHasEvent(t, collected, EventNodeCompleted, "a")
	assertHasEvent(t, collected, EventWorkflowCompleted, "")
}

func drainEvents(engine *WorkflowEngine) []WorkflowEvent {
	var events []WorkflowEvent
	for {
		select {
		case ev := <-engine.Events():
			events = append(events, ev)
		default:
			return events
		}
	}
}

func assertHasEvent(t *testing.T, events []WorkflowEvent, eventType WorkflowEventType, nodeID string) {
	t.Helper()
	for _, ev := range events {
		if ev.Type == eventType && (nodeID == "" || ev.NodeID == nodeID) {
			return
		}
	}
	t.Errorf("expected event %s", eventType)
}
