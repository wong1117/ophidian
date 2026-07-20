package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ophidian/ophidian/internal/application/policy"
	"github.com/ophidian/ophidian/internal/application/recommendation"
	"github.com/ophidian/ophidian/internal/domain/common"
	domainPolicy "github.com/ophidian/ophidian/internal/domain/policy"
	"github.com/ophidian/ophidian/internal/domain/finding"
	"github.com/ophidian/ophidian/internal/infrastructure/ha"
	"github.com/ophidian/ophidian/internal/infrastructure/queue"
	"github.com/ophidian/ophidian/internal/infrastructure/scheduler"
	"github.com/ophidian/ophidian/internal/infrastructure/secrets"
)

type testRepo struct {
	policies map[string]*domainPolicy.Policy
}

func (r *testRepo) Save(ctx context.Context, p *domainPolicy.Policy) error { r.policies[p.ID.String()] = p; return nil }
func (r *testRepo) FindByID(ctx context.Context, id string) (*domainPolicy.Policy, error) { return nil, nil }
func (r *testRepo) FindByName(ctx context.Context, name string) (*domainPolicy.Policy, error) { return nil, nil }
func (r *testRepo) FindAll(ctx context.Context) ([]*domainPolicy.Policy, error) { return nil, nil }
func (r *testRepo) FindEnabled(ctx context.Context) ([]*domainPolicy.Policy, error) { return nil, nil }
func (r *testRepo) Update(ctx context.Context, p *domainPolicy.Policy) error { return nil }
func (r *testRepo) Delete(ctx context.Context, id string) error { return nil }

func main() {
	fmt.Println("=== Ophidian Examples ===")

	examplePolicyEngine()
	exampleQueue()
	exampleScheduler()
	exampleHealthCheck()
	exampleSecretManager()
	exampleRecommendation()
}

func examplePolicyEngine() {
	fmt.Println("\n--- Policy Engine ---")
	svc := policy.NewPolicyService(&testRepo{policies: make(map[string]*domainPolicy.Policy)}, nil)
	ctx := context.Background()

	svc.Create(ctx, "example-policy", "Example policy for read-only access",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{Actions: []string{"read"}}, Decision: domainPolicy.DecisionAllow, Reason: "Allow read operations"},
			{Condition: domainPolicy.Condition{Actions: []string{"delete"}}, Decision: domainPolicy.DecisionDeny, Reason: "Block dangerous operations"},
		},
		domainPolicy.DecisionDeny, 10)

	result, _ := svc.Evaluate(ctx, &domainPolicy.EvaluationContext{
		ResourceType: "mission", Action: "read", Subject: "user-1",
	})
	fmt.Printf("  Policy evaluation for 'read': allowed=%v, decision=%s\n", result.Allowed, result.Decision)

	result2, _ := svc.Evaluate(ctx, &domainPolicy.EvaluationContext{
		ResourceType: "mission", Action: "delete", Subject: "user-1",
	})
	fmt.Printf("  Policy evaluation for 'delete': allowed=%v, decision=%s\n", result2.Allowed, result2.Decision)
}

func exampleQueue() {
	fmt.Println("\n--- Priority Queue ---")
	q := queue.NewPriorityQueue(nil)
	ctx := context.Background()
	_ = ctx

	q.Enqueue(&queue.Job{ID: "critical-fix", Handler: "patch", Priority: 100})
	q.Enqueue(&queue.Job{ID: "routine-scan", Handler: "scan", Priority: 10})
	q.Enqueue(&queue.Job{ID: "medium-alert", Handler: "alert", Priority: 50})

	for q.Size() > 0 {
		job, _ := q.Dequeue(nil)
		fmt.Printf("  Processing: %s (priority: %d)\n", job.ID, job.Priority)
		q.Ack(nil, job.ID)
	}
}

func exampleScheduler() {
	fmt.Println("\n--- Scheduler ---")
	s := scheduler.NewScheduler(nil)
	ctx := context.Background()
	_ = ctx

	executed := make(chan string, 3)
	s.Schedule(&scheduler.Job{
		ID: "quick-task", ScheduleType: scheduler.ScheduleOnce,
		RunAt: time.Now().Add(10 * time.Millisecond),
		Func:  func(ctx context.Context) error { executed <- "quick-task"; return nil },
	})
	s.Schedule(&scheduler.Job{
		ID: "recurring-task", ScheduleType: scheduler.ScheduleRecurring,
		Interval: 20 * time.Millisecond,
		Func:     func(ctx context.Context) error { executed <- "recurring-task"; return nil },
	})

	s.Start()
	defer s.Stop()
	time.Sleep(150 * time.Millisecond)
	close(executed)

	count := 0
	for range executed {
		count++
	}
	fmt.Printf("  Executed %d scheduled jobs\n", count)
}

func exampleHealthCheck() {
	fmt.Println("\n--- Health Check ---")
	hc := ha.NewHealthChecker()
	hc.Register(&simpleChecker{name: "database", healthy: true})
	hc.Register(&simpleChecker{name: "redis", healthy: true})

	results := hc.CheckAll(context.Background())
	for name, status := range results {
		fmt.Printf("  %s: healthy=%v\n", name, status.Healthy)
	}
	fmt.Printf("  Overall: healthy=%v\n", hc.IsHealthy())
}

type simpleChecker struct {
	name    string
	healthy bool
}

func (c *simpleChecker) Name() string                       { return c.name }
func (c *simpleChecker) Check(ctx context.Context) error {
	if !c.healthy { return fmt.Errorf("unhealthy") }
	return nil
}

func exampleSecretManager() {
	fmt.Println("\n--- Secret Manager ---")
	mgr := secrets.NewSecretManager(secrets.NewMemoryProvider(), "my-256-bit-encryption-key-here!")
	ctx := context.Background()

	mgr.Set(ctx, "api-key", "sk-abc123secret")
	val, _ := mgr.Get(ctx, "api-key")
	masked := val[:4] + "..." + val[len(val)-4:]
	fmt.Printf("  Stored API key: %s\n", masked)

	exists, _ := mgr.Exists(context.Background(), "api-key")
	fmt.Printf("  API key exists: %v\n", exists)

	mgr.Rotate(ctx, "api-key", "sk-xyz789newkey")
	newVal, _ := mgr.Get(ctx, "api-key")
	maskedNew := newVal[:4] + "..." + newVal[len(newVal)-4:]
	fmt.Printf("  Rotated API key: %s\n", maskedNew)

	oldVal, _ := mgr.Get(ctx, "api-key.prev")
	maskedOld := oldVal[:4] + "..." + oldVal[len(oldVal)-4:]
	fmt.Printf("  Previous API key: %s\n", maskedOld)
}

func exampleRecommendation() {
	fmt.Println("\n--- Recommendation Engine ---")
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

	result, _ := svc.Generate(context.Background(), input)
	fmt.Printf("  Generated %d recommendations:\n", len(result))
	for i, rec := range result[:3] {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"title":      rec.Title,
			"priority":   rec.Priority,
			"confidence": fmt.Sprintf("%.0f%%", rec.Confidence*100),
			"severity":   rec.Severity,
		}, "  ", "  ")
		fmt.Printf("  [%d] %s\n", i+1, string(data))
	}
}
