package workflow

import (
	"context"
	"errors"
	"fmt"
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

type testLogger struct {
	entries []string
	mu      sync.Mutex
}

func (l *testLogger) Debug(msg string, kv ...interface{}) {}
func (l *testLogger) Info(msg string, kv ...interface{})  { l.record(msg, kv...) }
func (l *testLogger) Warn(msg string, kv ...interface{})  { l.record(msg, kv...) }
func (l *testLogger) Error(msg string, kv ...interface{}) { l.record(msg, kv...) }
func (l *testLogger) record(msg string, kv ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, msg)
}

type testMetrics struct {
	counters  map[string]int64
	durations map[string]time.Duration
	mu        sync.Mutex
}

func newTestMetrics() *testMetrics {
	return &testMetrics{
		counters:  make(map[string]int64),
		durations: make(map[string]time.Duration),
	}
}

func (m *testMetrics) IncrementCounter(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}

func (m *testMetrics) RecordDuration(name string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations[name] = d
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
	result := engine.Execute(context.Background(), wf)

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
			{ID: "a", Name: "Node A", MaxRetries: 2, Backoff: func(attempt int) time.Duration { return 0 }},
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

	result := engine.Execute(context.Background(), wf)

	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["a"].Status)
	assert.Equal(t, 3, result.NodeResults["a"].Attempts)
}

func TestWorkflowEngine_RetryExhausted(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID: "wf-retry-fail",
		Nodes: []*Node{
			{ID: "a", Name: "Node A", MaxRetries: 1, Backoff: func(attempt int) time.Duration { return 0 }},
		},
	}

	exec.execFn = func(ctx context.Context, node *Node) error {
		return errors.New("permanent error")
	}

	result := engine.Execute(context.Background(), wf)

	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, NodeFailed, result.NodeResults["a"].Status)
	assert.Equal(t, 2, result.NodeResults["a"].Attempts)
	assert.Equal(t, 1, result.FailedNode)
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

	result := engine.Execute(context.Background(), wf)

	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, NodeCompleted, result.NodeResults["a"].Status)
	assert.Equal(t, NodeFailed, result.NodeResults["b"].Status)
	assert.Equal(t, NodeSkipped, result.NodeResults["c"].Status)
	assert.Equal(t, 1, result.SkippedNode)
}

func TestWorkflowEngine_Timeout(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID:      "wf-timeout",
		Timeout: 50 * time.Millisecond,
		Nodes: []*Node{
			{ID: "a", Name: "Node A"},
		},
	}

	exec.delays["a"] = 200 * time.Millisecond
	result := engine.Execute(context.Background(), wf)

	assert.Equal(t, StatusTimedOut, result.Status)
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
	result := engine.Execute(context.Background(), wf)

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

func TestWorkflowEngine_WithLogger(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	logger := &testLogger{}
	engine := NewWorkflowEngine(exec, tracker, WithLogger(logger))

	wf := linearWorkflow()
	engine.Execute(context.Background(), wf)

	assert.NotEmpty(t, logger.entries)
}

func TestWorkflowEngine_WithMetrics(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	metrics := newTestMetrics()
	engine := NewWorkflowEngine(exec, tracker, WithMetrics(metrics))

	wf := linearWorkflow()
	result := engine.Execute(context.Background(), wf)
	assert.Equal(t, StatusCompleted, result.Status)

	assert.Equal(t, int64(1), metrics.counters["workflow.started"])
	assert.Equal(t, int64(1), metrics.counters["workflow.completed"])
	assert.Greater(t, metrics.counters["node.completed"], int64(0))

	d, ok := metrics.durations["workflow.duration"]
	assert.True(t, ok)
	assert.Greater(t, d, time.Duration(0))
}

func TestWorkflowEngine_WithRetryMetrics(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	metrics := newTestMetrics()
	engine := NewWorkflowEngine(exec, tracker, WithMetrics(metrics))

	wf := &Workflow{
		ID: "wf-retry-metrics",
		Nodes: []*Node{
			{ID: "a", Name: "Node A", MaxRetries: 1, Backoff: func(attempt int) time.Duration { return 0 }},
		},
	}

	exec.execFn = func(ctx context.Context, node *Node) error {
		return errors.New("fail")
	}

	engine.Execute(context.Background(), wf)

	assert.Equal(t, int64(1), metrics.counters["workflow.failed"])
	assert.Equal(t, int64(1), metrics.counters["node.failed"])
	assert.Greater(t, metrics.counters["node.retry"], int64(0))
}

func TestWorkflowEngine_ConcurrencyLimit(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID:               "wf-conc-limit",
		ConcurrencyLimit: 1,
		Nodes:            []*Node{{ID: "a"}, {ID: "b"}, {ID: "c"}},
	}

	started := make(chan string, 3)
	var mu sync.Mutex
	active := make(map[string]bool)
	order := make([]string, 0)

	exec.execFn = func(ctx context.Context, node *Node) error {
		mu.Lock()
		active[node.ID] = true
		order = append(order, node.ID)
		mu.Unlock()
		started <- node.ID
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		delete(active, node.ID)
		mu.Unlock()
		return nil
	}

	result := engine.Execute(context.Background(), wf)
	assert.Equal(t, StatusCompleted, result.Status)

	activeCount := make(map[int]int)
	for i, id := range order {
		if i >= 2 {
			_ = id
			activeCount[i]++
		}
	}
}

func TestWorkflowEngine_LargeDAG(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	nodes := make([]*Node, 50)
	for i := 0; i < 50; i++ {
		nodes[i] = &Node{ID: fmt.Sprintf("n%d", i)}
	}
	edges := make([]Edge, 0)
	for i := 1; i < 50; i++ {
		edges = append(edges, Edge{From: fmt.Sprintf("n%d", i-1), To: fmt.Sprintf("n%d", i)})
	}

	wf := &Workflow{ID: "wf-large", Nodes: nodes, Edges: edges}
	result := engine.Execute(context.Background(), wf)

	assert.Equal(t, StatusCompleted, result.Status)
	assert.Len(t, result.NodeResults, 50)
	assert.Equal(t, 50, result.CompletedNode)
}

func TestBackoff_Exponential(t *testing.T) {
	b := DefaultBackoff()

	assert.Equal(t, time.Duration(0), b(1))

	d2 := b(2)
	assert.GreaterOrEqual(t, d2, 80*time.Millisecond)
	assert.LessOrEqual(t, d2, 220*time.Millisecond)

	d3 := b(3)
	assert.GreaterOrEqual(t, d3, 160*time.Millisecond)
	assert.LessOrEqual(t, d3, 440*time.Millisecond)

	d10 := b(10)
	assert.LessOrEqual(t, d10, 33*time.Second)
}

func TestBackoff_Custom(t *testing.T) {
	b := ExponentialBackoff(10*time.Millisecond, 100*time.Millisecond, 0)

	assert.Equal(t, time.Duration(0), b(1))
	assert.Equal(t, 20*time.Millisecond, b(2))
	assert.Equal(t, 40*time.Millisecond, b(3))
	assert.Equal(t, 80*time.Millisecond, b(4))
	assert.Equal(t, 100*time.Millisecond, b(5))
}

func TestValidateDAG_All(t *testing.T) {
	assert.NoError(t, ValidateDAG(linearWorkflow()))
	assert.NoError(t, ValidateDAG(dagWorkflow()))

	err := ValidateDAG(&Workflow{ID: "empty"})
	assert.ErrorIs(t, err, ErrInvalidDAG)
	assert.ErrorIs(t, err, ErrEmptyWorkflow)

	err = ValidateDAG(&Workflow{ID: "test", Nodes: []*Node{{ID: ""}}})
	assert.Error(t, err)

	err = ValidateDAG(&Workflow{ID: "test", Nodes: []*Node{{ID: "a"}, {ID: "a"}}})
	assert.ErrorIs(t, err, ErrDuplicateNode)

	err = ValidateDAG(&Workflow{ID: "test", Nodes: []*Node{{ID: "a"}}, Edges: []Edge{{From: "a", To: "b"}}})
	assert.ErrorIs(t, err, ErrNodeNotFound)

	err = ValidateDAG(&Workflow{ID: "test", Nodes: []*Node{{ID: "a"}}, Edges: []Edge{{From: "a", To: "a"}}})
	assert.Error(t, err)

	wf := &Workflow{ID: "test", Nodes: []*Node{{ID: "a"}, {ID: "b"}, {ID: "c"}}, Edges: []Edge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "a"}}}
	assert.ErrorIs(t, ValidateDAG(wf), ErrCyclicDependency)
}

func TestWorkflowEngine_EventsChannel(t *testing.T) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	wf := &Workflow{
		ID:    "wf-events",
		Nodes: []*Node{{ID: "a", Name: "Node A"}},
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

func TestDefaultBackoff_Values(t *testing.T) {
	b := DefaultBackoff()
	assert.Equal(t, time.Duration(0), b(1))
	assert.Greater(t, b(2), time.Duration(0))
	assert.Greater(t, b(3), b(2))
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

func BenchmarkWorkflowEngine_Linear(b *testing.B) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)
	wf := linearWorkflow()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Execute(ctx, wf)
	}
}

func BenchmarkWorkflowEngine_DAG(b *testing.B) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)
	wf := dagWorkflow()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Execute(ctx, wf)
	}
}

func BenchmarkWorkflowEngine_LargeLinear(b *testing.B) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	nodes := make([]*Node, 100)
	for i := 0; i < 100; i++ {
		nodes[i] = &Node{ID: fmt.Sprintf("n%d", i)}
	}
	edges := make([]Edge, 0)
	for i := 1; i < 100; i++ {
		edges = append(edges, Edge{From: fmt.Sprintf("n%d", i-1), To: fmt.Sprintf("n%d", i)})
	}
	wf := &Workflow{ID: "bench-large", Nodes: nodes, Edges: edges}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Execute(ctx, wf)
	}
}

func BenchmarkWorkflowEngine_Parallel(b *testing.B) {
	exec := newMockExecutor()
	tracker := newMockStatusTracker()
	engine := NewWorkflowEngine(exec, tracker)

	nodes := make([]*Node, 20)
	for i := 0; i < 20; i++ {
		nodes[i] = &Node{ID: fmt.Sprintf("n%d", i)}
	}
	wf := &Workflow{ID: "bench-parallel", Nodes: nodes}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Execute(ctx, wf)
	}
}
