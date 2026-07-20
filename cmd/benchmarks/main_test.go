package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/ophidian/ophidian/internal/application/graph"
	"github.com/ophidian/ophidian/internal/application/recommendation"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	domainGraph "github.com/ophidian/ophidian/internal/domain/graph"
	"github.com/ophidian/ophidian/internal/infrastructure/config"
	"github.com/ophidian/ophidian/internal/infrastructure/ha"
	"github.com/ophidian/ophidian/internal/infrastructure/persistence/redis"
	"github.com/ophidian/ophidian/internal/infrastructure/queue"
	"github.com/ophidian/ophidian/internal/infrastructure/scheduler"
	"github.com/ophidian/ophidian/internal/infrastructure/secrets"
	"github.com/ophidian/ophidian/internal/infrastructure/workflow"
)

type testGraphRepo = graph.TestGraphRepo

func BenchmarkSuite_FullPipeline(b *testing.B) {
	b.Run("EventStoreWrite", benchEventStoreWrite)
	b.Run("QueueEnqueue", benchQueueEnqueue)
	b.Run("QueueDequeue", benchQueueDequeue)
	b.Run("SchedulerSchedule", benchSchedulerSchedule)
	b.Run("WorkflowLinear", benchWorkflowLinear)
	b.Run("WorkflowDAG", benchWorkflowDAG)
	b.Run("CacheSet", benchCacheSet)
	b.Run("CacheGet", benchCacheGet)
	b.Run("GraphShortestPath", benchGraphShortestPath)
	b.Run("GraphTraverse", benchGraphTraverse)
	b.Run("SecretEncrypt", benchSecretEncrypt)
	b.Run("SecretDecrypt", benchSecretDecrypt)
	b.Run("RecEngineGenerate", benchRecEngineGenerate)
	b.Run("ConfigLoad", benchConfigLoad)
	b.Run("SerializationJSON", benchSerializationJSON)
	b.Run("HealthCheck", benchHealthCheck)
}

func benchEventStoreWrite(b *testing.B) {
	type event struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	evt := event{ID: "evt-1", Type: "MissionCreated"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = evt
	}
}

func benchQueueEnqueue(b *testing.B) {
	q := queue.NewPriorityQueue(nil)
	job := &queue.Job{ID: "bench-job", Handler: "test", Priority: 50}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(job)
		job.ID = fmt.Sprintf("bench-%d", i)
	}
}

func benchQueueDequeue(b *testing.B) {
	q := queue.NewPriorityQueue(nil)
	for i := 0; i < 10000; i++ {
		q.Enqueue(&queue.Job{ID: fmt.Sprintf("dq-%d", i), Handler: "test"})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j, _ := q.Dequeue(nil)
		if j != nil {
			q.Ack(nil, j.ID)
			q.Enqueue(&queue.Job{ID: fmt.Sprintf("replenish-%d", i), Handler: "test"})
		}
	}
}

func benchSchedulerSchedule(b *testing.B) {
	s := scheduler.NewScheduler(nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Schedule(&scheduler.Job{
			ID:           fmt.Sprintf("bench-sched-%d", i),
			ScheduleType: scheduler.ScheduleOnce,
			RunAt:        time.Now().Add(time.Hour),
			Func:         func(ctx context.Context) error { return nil },
		})
	}
}

func benchWorkflowLinear(b *testing.B) {
	wf := &workflow.Workflow{
		ID: "bench-wf",
		Nodes: []*workflow.Node{
			{ID: "a"}, {ID: "b"}, {ID: "c"},
		},
		Edges: []workflow.Edge{{From: "a", To: "b"}, {From: "b", To: "c"}},
	}
	type tExec struct{}
	exec := &benchExecutor{}
	engine := workflow.NewWorkflowEngine(exec, &benchTracker{})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Execute(context.Background(), wf)
	}
}

type benchExecutor struct{}

func (e *benchExecutor) Execute(ctx context.Context, node *workflow.Node) error { return nil }

type benchTracker struct{}

func (t *benchTracker) SaveStatus(ctx context.Context, id string, status workflow.WorkflowStatus) error {
	return nil
}
func (t *benchTracker) SaveNodeResult(ctx context.Context, wfID, nodeID string, result workflow.NodeResult) error {
	return nil
}

func benchWorkflowDAG(b *testing.B) {
	wf := &workflow.Workflow{
		ID: "bench-dag",
		Nodes: []*workflow.Node{
			{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
		},
		Edges: []workflow.Edge{{From: "a", To: "c"}, {From: "b", To: "c"}, {From: "c", To: "d"}},
	}
	engine := workflow.NewWorkflowEngine(&benchExecutor{}, &benchTracker{})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Execute(context.Background(), wf)
	}
}

func benchCacheSet(b *testing.B) {
	c := redis.NewMemoryCache()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, fmt.Sprintf("bench-key-%d", i%1000), "value", time.Hour)
	}
}

func benchCacheGet(b *testing.B) {
	c := redis.NewMemoryCache()
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		c.Set(ctx, fmt.Sprintf("gk-%d", i), "val", time.Hour)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v string
		c.Get(ctx, fmt.Sprintf("gk-%d", i%10000), &v)
	}
}

func benchGraphShortestPath(b *testing.B) {
	repo := newBenchGraphRepo()
	svc := graph.NewGraphService(repo)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		repo.nodes[fmt.Sprintf("n%d", i)] = &domainGraph.Node{ID: common.ID(fmt.Sprintf("n%d", i)), EntityType: "t"}
	}
	for i := 0; i < 19; i++ {
		repo.edges = append(repo.edges, domainGraph.Edge{
			FromNodeID: common.ID(fmt.Sprintf("n%d", i)),
			ToNodeID:   common.ID(fmt.Sprintf("n%d", i+1)),
			Weight:     1,
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.ShortestPath(ctx, "n0", "n19")
	}
}

func benchGraphTraverse(b *testing.B) {
	repo := newBenchGraphRepo()
	svc := graph.NewGraphService(repo)
	ctx := context.Background()

	for i := 0; i < 50; i++ {
		repo.nodes[fmt.Sprintf("t%d", i)] = &domainGraph.Node{ID: common.ID(fmt.Sprintf("t%d", i)), EntityType: "t"}
	}
	for i := 0; i < 49; i++ {
		repo.edges = append(repo.edges, domainGraph.Edge{
			FromNodeID: common.ID(fmt.Sprintf("t%d", i)),
			ToNodeID:   common.ID(fmt.Sprintf("t%d", i+1)),
			Weight:     1,
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.Traverse(ctx, "t0", 50)
	}
}

type benchGraphRepo struct {
	nodes map[string]*domainGraph.Node
	edges []domainGraph.Edge
}

func newBenchGraphRepo() *benchGraphRepo {
	return &benchGraphRepo{
		nodes: make(map[string]*domainGraph.Node),
		edges: make([]domainGraph.Edge, 0),
	}
}

func (r *benchGraphRepo) SaveGraph(ctx context.Context, g *domainGraph.Graph) error       { return nil }
func (r *benchGraphRepo) FindGraphByID(ctx context.Context, id string) (*domainGraph.Graph, error) { return nil, nil }
func (r *benchGraphRepo) FindGraphsByName(ctx context.Context, name string) ([]*domainGraph.Graph, error) { return nil, nil }
func (r *benchGraphRepo) SaveNode(ctx context.Context, n *domainGraph.Node) error         { return nil }
func (r *benchGraphRepo) FindNodeByID(ctx context.Context, id string) (*domainGraph.Node, error) {
	return r.nodes[id], nil
}
func (r *benchGraphRepo) FindNodesByGraph(ctx context.Context, graphID string) ([]*domainGraph.Node, error) {
	var result []*domainGraph.Node
	for _, n := range r.nodes { result = append(result, n) }
	return result, nil
}
func (r *benchGraphRepo) FindNodesByEntity(ctx context.Context, et, eid string) ([]*domainGraph.Node, error) { return nil, nil }
func (r *benchGraphRepo) DeleteNode(ctx context.Context, id string) error               { return nil }
func (r *benchGraphRepo) SaveEdge(ctx context.Context, e *domainGraph.Edge) error        { return nil }
func (r *benchGraphRepo) FindEdgeByID(ctx context.Context, id string) (*domainGraph.Edge, error) { return nil, nil }
func (r *benchGraphRepo) FindEdgesByGraph(ctx context.Context, graphID string) ([]*domainGraph.Edge, error) {
	result := make([]*domainGraph.Edge, len(r.edges))
	for i := range r.edges { result[i] = &r.edges[i] }
	return result, nil
}
func (r *benchGraphRepo) FindOutgoingEdges(ctx context.Context, nodeID string) ([]*domainGraph.Edge, error) {
	var result []*domainGraph.Edge
	for i := range r.edges {
		if r.edges[i].FromNodeID.String() == nodeID {
			cp := r.edges[i]
			result = append(result, &cp)
		}
	}
	return result, nil
}
func (r *benchGraphRepo) FindIncomingEdges(ctx context.Context, nodeID string) ([]*domainGraph.Edge, error) { return nil, nil }
func (r *benchGraphRepo) DeleteEdge(ctx context.Context, id string) error               { return nil }

func benchSecretEncrypt(b *testing.B) {
	mgr := secrets.NewSecretManager(secrets.NewMemoryProvider(), "0123456789abcdef0123456789abcdef")
	plaintext := []byte("sensitive-data-for-benchmarking-purposes")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.EncryptForBench(plaintext)
	}
}

func benchSecretDecrypt(b *testing.B) {
	mgr := secrets.NewSecretManager(secrets.NewMemoryProvider(), "0123456789abcdef0123456789abcdef")
	ct, _ := mgr.EncryptForBench([]byte("sensitive-data-for-benchmarking-purposes"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.DecryptForBench(ct)
	}
}

func benchRecEngineGenerate(b *testing.B) {
	svc := recommendation.NewRecommendationService(nil)
	input := &recommendation.AssessmentInput{
		Findings: []finding.Finding{
			{ID: common.NewID(), Title: "SQL Injection", Severity: common.SeverityCritical, CVSS: 9.8, CVE: "CVE-2024-0001", Confidence: finding.ConfidenceConfirmed},
			{ID: common.NewID(), Title: "Outdated TLS", Severity: common.SeverityHigh, CVSS: 7.5, Confidence: finding.ConfidenceHigh},
			{ID: common.NewID(), Title: "Info Leak", Severity: common.SeverityLow, CVSS: 2.5, Confidence: finding.ConfidenceMedium},
		},
		AssetCriticality: 3,
		ComplianceReqs:    []string{"PCI-DSS", "SOC2"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.Generate(context.Background(), input)
	}
}

func benchConfigLoad(b *testing.B) {
	l := config.NewLoader("")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Load()
	}
}

func benchSerializationJSON(b *testing.B) {
	type ReportData struct {
		MissionID  string   `json:"mission_id"`
		Targets    []string `json:"targets"`
		Findings   int      `json:"findings"`
		Confidence float64  `json:"confidence"`
	}

	data := &ReportData{MissionID: "test-123", Targets: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "example.com"}, Findings: 42, Confidence: 0.875}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}

func benchHealthCheck(b *testing.B) {
	hc := ha.NewHealthChecker()
	hc.Register(&benchHealthProbe{name: "db"})
	hc.Register(&benchHealthProbe{name: "redis"})
	hc.Register(&benchHealthProbe{name: "ai"})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.CheckAll(ctx)
	}
}

type benchHealthProbe struct{ name string }

func (p *benchHealthProbe) Name() string               { return p.name }
func (p *benchHealthProbe) Check(ctx context.Context) error { return nil }

func BenchmarkRuntime_Metrics(b *testing.B) {
	b.Run("Goroutines", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			runtime.NumGoroutine()
		}
	})
	b.Run("MemStats", func(b *testing.B) {
		var m runtime.MemStats
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			runtime.ReadMemStats(&m)
		}
	})
	b.Run("GOMAXPROCS", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			runtime.GOMAXPROCS(0)
		}
	})
}
