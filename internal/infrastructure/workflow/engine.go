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
	ID               string
	Name             string
	Nodes            []*Node
	Edges            []Edge
	Timeout          time.Duration
	Backoff          BackoffFunc
	ConcurrencyLimit int
}

type Node struct {
	ID         string
	Name       string
	MaxRetries int
	Timeout    time.Duration
	Backoff    BackoffFunc
	Metadata   map[string]interface{}
}

type Edge struct {
	From string
	To   string
}

type NodeExecutor interface {
	Execute(ctx context.Context, node *Node) error
}

type Logger interface {
	Debug(msg string, kv ...interface{})
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
}

type Metrics interface {
	IncrementCounter(name string, value int64)
	RecordDuration(name string, d time.Duration)
}

type WorkflowEvent struct {
	WorkflowID string
	Type       WorkflowEventType
	NodeID     string
	Status     NodeStatus
	Error      string
	Timestamp  time.Time
	Attempt    int
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
	SkippedNode   int
}

type NodeResult struct {
	Status      NodeStatus
	Error       string
	Attempts    int
	Duration    time.Duration
	StartedAt   time.Time
	CompletedAt time.Time
}

type WorkflowEngine struct {
	executor      NodeExecutor
	logger        Logger
	metrics       Metrics
	eventCh       chan WorkflowEvent
	statusTracker StatusTracker
	mu            sync.RWMutex
}

type StatusTracker interface {
	SaveStatus(ctx context.Context, workflowID string, status WorkflowStatus) error
	SaveNodeResult(ctx context.Context, workflowID, nodeID string, result NodeResult) error
}

type EngineOption func(*WorkflowEngine)

func WithLogger(logger Logger) EngineOption {
	return func(e *WorkflowEngine) { e.logger = logger }
}

func WithMetrics(metrics Metrics) EngineOption {
	return func(e *WorkflowEngine) { e.metrics = metrics }
}

func NewWorkflowEngine(executor NodeExecutor, tracker StatusTracker, opts ...EngineOption) *WorkflowEngine {
	e := &WorkflowEngine{
		executor:      executor,
		eventCh:       make(chan WorkflowEvent, 100),
		statusTracker: tracker,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *WorkflowEngine) Events() <-chan WorkflowEvent {
	return e.eventCh
}

func (e *WorkflowEngine) Execute(ctx context.Context, wf *Workflow) *WorkflowResult {
	startTime := time.Now()
	result := &WorkflowResult{
		NodeResults: make(map[string]NodeResult),
	}

	e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowStarted, Timestamp: time.Now()})
	e.log(LevelInfo, "workflow started", "workflow_id", wf.ID, "nodes", len(wf.Nodes))
	e.metricCounter("workflow.started", 1)

	if err := ValidateDAG(wf); err != nil {
		result.Status = StatusFailed
		e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowFailed, Error: err.Error(), Timestamp: time.Now()})
		e.log(LevelError, "workflow validation failed", "workflow_id", wf.ID, "error", err.Error())
		e.metricCounter("workflow.validation_failed", 1)
		_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusFailed)
		return result
	}

	_ = e.statusTracker.SaveStatus(ctx, wf.ID, StatusRunning)

	if wf.Backoff == nil {
		wf.Backoff = DefaultBackoff()
	}
	concurrencyLimit := wf.ConcurrencyLimit
	if concurrencyLimit <= 0 {
		concurrencyLimit = len(wf.Nodes) * 2
	}

	execCtx, cancel := e.setupContext(ctx, wf)
	defer cancel()

	trigger := make(chan string, len(wf.Nodes))
	startNodes := e.getRootNodes(wf)
	go func() {
		for _, id := range startNodes {
			trigger <- id
		}
	}()

	resultsCh := make(chan nodeExecution, len(wf.Nodes))
	var active int
	sem := make(chan struct{}, concurrencyLimit)
	nodeMap := e.buildNodeMap(wf)

	for active > 0 || len(resultsCh) > 0 || len(wf.Nodes) > 0 {
		select {
		case nodeID := <-trigger:
			sem <- struct{}{}
			active++
			go func(id string) {
				defer func() { <-sem }()
				e.runNode(execCtx, wf, nodeMap[id], resultsCh)
			}(nodeID)

		case exec := <-resultsCh:
			active--
			e.processNodeResult(ctx, wf, exec, result, trigger)

			if len(result.NodeResults) == len(wf.Nodes) {
				goto done
			}

		case <-execCtx.Done():
			result.Status = e.handleContextDone(execCtx, wf)
			e.log(LevelWarn, "workflow context done", "workflow_id", wf.ID, "status", string(result.Status))
			goto done
		}
	}

done:
	result.Duration = time.Since(startTime)
	result.CompletedNode = countByStatus(result, NodeCompleted)
	result.FailedNode = countByStatus(result, NodeFailed)
	result.SkippedNode = countByStatus(result, NodeSkipped)

	if result.Status == "" {
		if result.FailedNode > 0 {
			result.Status = StatusFailed
			e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowFailed, Timestamp: time.Now()})
			e.log(LevelError, "workflow failed", "workflow_id", wf.ID, "completed", result.CompletedNode, "failed", result.FailedNode)
		} else {
			result.Status = StatusCompleted
			e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowCompleted, Timestamp: time.Now()})
			e.log(LevelInfo, "workflow completed", "workflow_id", wf.ID, "nodes", result.CompletedNode, "duration_ms", result.Duration.Milliseconds())
		}
		_ = e.statusTracker.SaveStatus(ctx, wf.ID, result.Status)
	}

	e.metricDuration("workflow.duration", result.Duration)
	if result.FailedNode > 0 {
		e.metricCounter("workflow.failed", 1)
	} else {
		e.metricCounter("workflow.completed", 1)
	}

	return result
}

func (e *WorkflowEngine) setupContext(ctx context.Context, wf *Workflow) (context.Context, context.CancelFunc) {
	if wf.Timeout > 0 {
		return context.WithTimeout(ctx, wf.Timeout)
	}
	return context.WithCancel(ctx)
}

func (e *WorkflowEngine) runNode(ctx context.Context, wf *Workflow, node *Node, resultsCh chan<- nodeExecution) {
	result := nodeExecution{nodeID: node.ID, startedAt: time.Now()}
	var execErr error

	backoff := node.Backoff
	if backoff == nil {
		backoff = wf.Backoff
	}

	for attempt := 1; attempt <= node.MaxRetries+1; attempt++ {
		nodeCtx := ctx
		var cancel context.CancelFunc
		if node.Timeout > 0 {
			nodeCtx, cancel = context.WithTimeout(ctx, node.Timeout)
		}

		e.emit(WorkflowEvent{WorkflowID: wf.ID, NodeID: node.ID, Type: EventNodeStarted, Timestamp: time.Now(), Attempt: attempt})
		e.log(LevelDebug, "node started", "workflow_id", wf.ID, "node", node.ID, "attempt", attempt)

		execErr = e.executor.Execute(nodeCtx, node)
		result.attempts = attempt
		if cancel != nil {
			cancel()
		}

		if execErr == nil {
			e.emit(WorkflowEvent{WorkflowID: wf.ID, NodeID: node.ID, Type: EventNodeCompleted, Timestamp: time.Now(), Attempt: attempt})
			e.log(LevelInfo, "node completed", "workflow_id", wf.ID, "node", node.ID, "attempt", attempt)
			break
		}

		if attempt <= node.MaxRetries {
			delay := backoff(attempt)
			e.emit(WorkflowEvent{WorkflowID: wf.ID, NodeID: node.ID, Type: EventNodeRetrying, Error: execErr.Error(), Timestamp: time.Now(), Attempt: attempt})
			e.log(LevelWarn, "node retrying", "workflow_id", wf.ID, "node", node.ID, "attempt", attempt, "delay_ms", delay.Milliseconds(), "error", execErr.Error())
			e.metricCounter("node.retry", 1)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				execErr = ctx.Err()
				result.attempts = attempt
				goto done
			}
		}
	}

done:
	result.err = execErr
	result.duration = time.Since(result.startedAt)
	resultsCh <- result
}

func (e *WorkflowEngine) processNodeResult(
	ctx context.Context,
	wf *Workflow,
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
		e.log(LevelError, "node failed", "workflow_id", wf.ID, "node", exec.nodeID, "error", exec.err.Error())
		e.metricCounter("node.failed", 1)

		for _, edge := range wf.Edges {
			if edge.From == exec.nodeID {
				depID := edge.To
				if _, exists := result.NodeResults[depID]; !exists {
					nr := NodeResult{Status: NodeSkipped, StartedAt: time.Now(), CompletedAt: time.Now()}
					result.NodeResults[depID] = nr
					e.emit(WorkflowEvent{WorkflowID: wf.ID, NodeID: depID, Type: EventNodeSkipped, Timestamp: time.Now()})
					_ = e.statusTracker.SaveNodeResult(ctx, wf.ID, depID, nr)
				}
			}
		}
	} else {
		nr.Status = NodeCompleted
		e.metricDuration("node.duration", exec.duration)
		e.metricCounter("node.completed", 1)
	}

	result.NodeResults[exec.nodeID] = nr
	_ = e.statusTracker.SaveNodeResult(ctx, wf.ID, exec.nodeID, nr)

	for _, edge := range wf.Edges {
		if edge.From == exec.nodeID {
			depID := edge.To
			if _, processed := result.NodeResults[depID]; processed {
				continue
			}
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
				trigger <- depID
			}
		}
	}
}

func (e *WorkflowEngine) handleContextDone(execCtx context.Context, wf *Workflow) WorkflowStatus {
	if execCtx.Err() == context.DeadlineExceeded {
		e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowTimedOut, Timestamp: time.Now()})
		e.metricCounter("workflow.timed_out", 1)
		return StatusTimedOut
	}
	e.emit(WorkflowEvent{WorkflowID: wf.ID, Type: EventWorkflowCancelled, Timestamp: time.Now()})
	e.metricCounter("workflow.cancelled", 1)
	return StatusCancelled
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

type logLevel string

const (
	LevelDebug logLevel = "DEBUG"
	LevelInfo  logLevel = "INFO"
	LevelWarn  logLevel = "WARN"
	LevelError logLevel = "ERROR"
)

func (e *WorkflowEngine) log(level logLevel, msg string, kv ...interface{}) {
	if e.logger == nil {
		return
	}
	switch level {
	case LevelDebug:
		e.logger.Debug(msg, kv...)
	case LevelInfo:
		e.logger.Info(msg, kv...)
	case LevelWarn:
		e.logger.Warn(msg, kv...)
	case LevelError:
		e.logger.Error(msg, kv...)
	}
}

func (e *WorkflowEngine) metricCounter(name string, value int64) {
	if e.metrics != nil {
		e.metrics.IncrementCounter(name, value)
	}
}

func (e *WorkflowEngine) metricDuration(name string, d time.Duration) {
	if e.metrics != nil {
		e.metrics.RecordDuration(name, d)
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
