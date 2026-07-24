package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	appai "github.com/ophidian/ophidian/internal/application/ai"
	"github.com/ophidian/ophidian/internal/application/cognitive"
	"github.com/ophidian/ophidian/internal/application/executionplan"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/execution"
	"github.com/ophidian/ophidian/internal/domain/mission"
	infraai "github.com/ophidian/ophidian/internal/infrastructure/ai"
	"github.com/ophidian/ophidian/internal/infrastructure/ai/providerfactory"
	"github.com/ophidian/ophidian/internal/infrastructure/dispatcher"
	"github.com/ophidian/ophidian/internal/infrastructure/persistence/postgres"
	"github.com/ophidian/ophidian/internal/infrastructure/plugins"
	"github.com/ophidian/ophidian/internal/infrastructure/queue"
	"github.com/ophidian/ophidian/internal/infrastructure/runner"
	pkgexploit "github.com/ophidian/ophidian/pkg/exploit"
)

func main() {
	log.Println("=== ophidian-worker starting ===")
	loadDotEnv(".env")

	pool := connectDB()
	defer pool.Close()
	missionRepo := postgres.NewMissionRepository(pool)
	eventStore := postgres.NewEventStore(pool)
	eventStore.Migrate(context.Background())

	mux := http.NewServeMux()

	nmapRunner := runner.NewNmapRunner()
	nucleiPlugin := runner.NewNucleiPlugin()
	subfinderPlugin := runner.NewSubfinderPlugin()
	httpxPlugin := runner.NewHttpxPlugin()
	gobusterPlugin := runner.NewGobusterPlugin()
	httpProbePlugin := runner.NewHTTPProbePlugin()
	registry := plugins.NewToolRegistry()
	registry.Register(nmapRunner)
	registry.Register(nucleiPlugin)
	registry.Register(subfinderPlugin)
	registry.Register(httpxPlugin)
	registry.Register(gobusterPlugin)
	registry.Register(httpProbePlugin)

	q := queue.NewPriorityQueue(nil, queue.WithQueueLogger(stdLogger{}))
	worker := NewWorker(q, missionRepo, registry, eventStore)

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var envelope dispatcher.EventEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			http.Error(w, fmt.Sprintf("invalid event: %v", err), http.StatusBadRequest)
			return
		}

		log.Printf("WORKER: event received: type=%s aggregate=%s", envelope.EventType, envelope.AggregateID)

		job := &queue.Job{
			ID:      fmt.Sprintf("job-%s-%d", envelope.AggregateID, time.Now().UnixNano()),
			Handler: envelope.EventType,
			Payload: envelope,
		}

		if err := q.Enqueue(job); err != nil {
			http.Error(w, fmt.Sprintf("enqueue failed: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("WORKER: job enqueued: id=%s handler=%s", job.ID, job.Handler)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"accepted"}`))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","pending":%d,"inflight":%d}`, q.Pending(), q.InFlight())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.Run(ctx)
	go startAIEventSubscriber(ctx, eventStore)

	srv := &http.Server{Addr: ":9090", Handler: mux}
	go func() {
		log.Println("WORKER: listening on :9090")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("WORKER: shutting down...")
	cancel()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	srv.Shutdown(shutdownCtx)
}

type Worker struct {
	q           *queue.PriorityQueue
	missionRepo *postgres.MissionRepository
	registry    *plugins.ToolRegistry
	eventStore  *postgres.EventStore
}

func NewWorker(q *queue.PriorityQueue, repo *postgres.MissionRepository, registry *plugins.ToolRegistry, es *postgres.EventStore) *Worker {
	return &Worker{q: q, missionRepo: repo, registry: registry, eventStore: es}
}

func (w *Worker) Run(ctx context.Context) {
	log.Println("WORKER: event loop started, polling queue...")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("WORKER: event loop stopped")
			return
		case <-ticker.C:
			w.processNext(ctx)
		}
	}
}

func (w *Worker) processNext(ctx context.Context) {
	job, err := w.q.Dequeue(ctx)
	if err != nil {
		log.Printf("WORKER: dequeue error: %v", err)
		return
	}
	if job == nil {
		return
	}

	log.Printf("WORKER: processing job id=%s handler=%s payload=%v", job.ID, job.Handler, job.Payload)

	switch job.Handler {
	case "MissionStarted":
		w.handleMissionStarted(job)
	case "MissionStateChanged":
		w.handleStateChanged(job)
	case "PhaseTransitioned":
		w.handlePhaseTransitioned(job)
	case "TaskDispatched":
		w.handleTaskDispatched(job)
	case "OperatorApproval":
		w.handleOperatorApproval(job)
	default:
		log.Printf("WORKER: unknown event type: %s", job.Handler)
	}

	if err := w.q.Ack(ctx, job.ID); err != nil {
		log.Printf("WORKER: ack error: %v", err)
	}

	log.Printf("WORKER: job completed: id=%s handler=%s", job.ID, job.Handler)
}

func (w *Worker) handleMissionStarted(job *queue.Job) {
	envelope, ok := job.Payload.(dispatcher.EventEnvelope)
	if !ok {
		log.Println("WORKER: MissionStarted: invalid payload type")
		return
	}

	var payload struct {
		MissionID string `json:"MissionID"`
		StartedBy string `json:"StartedBy"`
	}
	json.Unmarshal(envelope.Payload, &payload)

	log.Printf("WORKER: ========================================")
	log.Printf("WORKER: MISSION STARTED!")
	log.Printf("WORKER:   mission_id=%s", payload.MissionID)
	log.Printf("WORKER: ========================================")

	m, err := w.missionRepo.FindByID(context.Background(), payload.MissionID)
	if err != nil {
		log.Printf("WORKER: WARNING: failed to load aggregate state for mission %s: %v", payload.MissionID, err)
		log.Printf("WORKER: -> WARNING: target details unavailable, mission may fail")
		return
	}

	targets := m.Target.Domains
	if len(m.Target.IPs) > 0 {
		targets = append(targets, m.Target.IPs...)
	}

	log.Printf("WORKER: -> mission loaded: name=%q domains=%v ips=%v", m.Name, m.Target.Domains, m.Target.IPs)
	log.Printf("WORKER: -> preparing reconnaissance for %d target(s): %v", len(targets), targets)

	parallel := envOrInt("WORKER_PARALLELISM", runtime.NumCPU())
	if parallel < 1 {
		parallel = 1
	}
	ratePerSec := envOrInt("WORKER_RATE_PER_SEC", 2)
	log.Printf("WORKER: -> scanning %d target(s) with parallelism=%d rate=%d/s", len(targets), parallel, ratePerSec)

	rateLimiter := time.NewTicker(time.Second / time.Duration(ratePerSec))
	defer rateLimiter.Stop()

	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			<-rateLimiter.C
			sem <- struct{}{}
			defer func() { <-sem }()
			w.runReconForTarget(common.ID(payload.MissionID), t)
		}(target)
	}
	wg.Wait()
	log.Printf("WORKER: -> all %d target(s) completed for mission %s", len(targets), payload.MissionID)

	if strings.ToLower(os.Getenv("WORKER_MODE")) == "adaptive" {
		for _, target := range targets {
			w.runAdaptiveLoop(payload.MissionID, target)
		}
		log.Printf("WORKER: -> adaptive loop completed for mission %s", payload.MissionID)
	}
}

func (w *Worker) runReconForTarget(missionID common.ID, target string) {
	startedAt := common.Now()
	log.Printf("WORKER: -> scanning: %s", target)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	tools := w.registry.GetAll()
	results := make([]execution.ToolResult, 0, len(tools))

	for _, tool := range tools {
		toolName := tool.Name()
		log.Printf("WORKER: -> running tool: %s on %s", toolName, target)

		result, err := tool.Run(ctx, execution.ToolRequest{
			MissionID: missionID.String(),
			Target:    target,
			Options: execution.ToolOptions{
				Timeout: 0,
			},
			Metadata: map[string]string{
				"source": "ophidian-worker",
			},
		})

		if err != nil {
			log.Printf("WORKER: -> tool %s FAILED for %s: %v", toolName, target, err)
			results = append(results, execution.ToolResult{
				Evidence: fmt.Sprintf("tool %s failed: %v", toolName, err),
				Metadata: map[string]string{
					"tool":   toolName,
					"target": target,
					"status": "failed",
				},
			})
			continue
		}

		if result == nil {
			log.Printf("WORKER: -> tool %s returned nil result for %s", toolName, target)
			results = append(results, execution.ToolResult{
				Evidence: fmt.Sprintf("tool %s returned no result", toolName),
				Metadata: map[string]string{
					"tool":   toolName,
					"target": target,
					"status": "empty",
				},
			})
			continue
		}

		evidenceLen := len(result.Evidence)
		log.Printf("WORKER: -> tool %s COMPLETE for %s (%d bytes)", toolName, target, evidenceLen)
		results = append(results, *result)
	}

	completedAt := common.Now()
	totalEvidence := 0
	for _, r := range results {
		totalEvidence += len(r.Evidence)
	}

	var status common.TaskStatus
	if totalEvidence > 0 {
		status = common.TaskSuccess
	} else {
		status = common.TaskFailed
	}

	log.Printf("WORKER: ========================================")
	log.Printf("WORKER: Multi-Tool Recon Complete")
	log.Printf("WORKER:   mission_id:  %s", missionID)
	log.Printf("WORKER:   target:      %s", target)
	log.Printf("WORKER:   tools_run:   %d", len(results))
	log.Printf("WORKER:   status:      %s", status)
	log.Printf("WORKER:   total_bytes: %d", totalEvidence)
	log.Printf("WORKER:   started:     %s", startedAt.Time.Format(time.RFC3339))
	log.Printf("WORKER:   completed:   %s", completedAt.Time.Format(time.RFC3339))
	for _, r := range results {
		toolName := r.Metadata["tool"]
		log.Printf("WORKER:   - tool=%s evidence=%d bytes", toolName, len(r.Evidence))
	}
	log.Printf("WORKER: ========================================")

	legacyEvent := mission.ReconCompletedEvent{
		MissionID:   missionID,
		Target:      target,
		RawOutput:   buildLegacyRawOutput(results),
		Status:      status,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}

	if w.eventStore != nil {
		payloadBytes, _ := json.Marshal(legacyEvent)
		record := postgres.EventRecord{
			ID:            legacyEvent.EventID(),
			AggregateID:   legacyEvent.AggregateID(),
			AggregateType: "mission",
			EventType:     legacyEvent.EventType(),
			Payload:       payloadBytes,
			OccurredAt:    legacyEvent.CompletedAt.Time,
		}
		if err := w.eventStore.Append(context.Background(), -1, record); err != nil {
			log.Printf("WORKER: WARNING: failed to append legacy recon event to store: %v", err)
		} else {
			log.Printf("WORKER: ReconCompletedEvent appended for mission %s", missionID)
		}

		multiEvent := execution.MultiToolReconCompletedEvent{
			Result: execution.MultiToolReconResult{
				MissionID:   missionID.String(),
				Target:      target,
				StartedAt:   startedAt,
				CompletedAt: completedAt,
				Duration:    completedAt.Sub(startedAt),
				Results:     results,
			},
		}
		multiPayload, _ := json.Marshal(multiEvent)
		multiRecord := postgres.EventRecord{
			ID:            multiEvent.EventID(),
			AggregateID:   multiEvent.AggregateID(),
			AggregateType: "mission",
			EventType:     multiEvent.EventType(),
			Payload:       multiPayload,
			OccurredAt:    multiEvent.OccurredAt().Time,
		}
		if err := w.eventStore.Append(context.Background(), -1, multiRecord); err != nil {
			log.Printf("WORKER: WARNING: failed to append multi-tool recon event to store: %v", err)
		} else {
			log.Printf("WORKER: MultiToolReconCompleted event appended for mission %s target %s", missionID, target)
		}
	}
}

func buildLegacyRawOutput(results []execution.ToolResult) string {
	var sb strings.Builder
	for _, r := range results {
		toolName := r.Metadata["tool"]
		if toolName == "" {
			toolName = "unknown"
		}
		fmt.Fprintf(&sb, "=== %s ===\n%s\n\n", toolName, r.Evidence)
	}
	return strings.TrimSpace(sb.String())
}

func (w *Worker) handleStateChanged(job *queue.Job) {
	envelope, _ := job.Payload.(dispatcher.EventEnvelope)
	log.Printf("WORKER: MissionStateChanged: agg=%s payload=%s", envelope.AggregateID, string(envelope.Payload))
}

func (w *Worker) handlePhaseTransitioned(job *queue.Job) {
	envelope, _ := job.Payload.(dispatcher.EventEnvelope)
	log.Printf("WORKER: PhaseTransitioned: agg=%s payload=%s", envelope.AggregateID, string(envelope.Payload))
}

func (w *Worker) handleTaskDispatched(job *queue.Job) {
	envelope, _ := job.Payload.(dispatcher.EventEnvelope)
	log.Printf("WORKER: TaskDispatched: agg=%s payload=%s", envelope.AggregateID, string(envelope.Payload))
}

func (w *Worker) handleOperatorApproval(job *queue.Job) {
	envelope, ok := job.Payload.(dispatcher.EventEnvelope)
	if !ok {
		log.Println("WORKER: OperatorApproval: invalid payload type")
		return
	}

	var approval mission.OperatorApprovalEvent
	if err := json.Unmarshal(envelope.Payload, &approval); err != nil {
		log.Printf("WORKER: OperatorApproval: failed to unmarshal: %v", err)
		return
	}

	log.Printf("WORKER: ========================================")
	log.Printf("WORKER: OPERATOR APPROVAL RECEIVED")
	log.Printf("WORKER:   mission_id:     %s", approval.MissionID)
	log.Printf("WORKER:   source_event:   %s", approval.SourceEventID)
	log.Printf("WORKER:   decision:       %s", approval.Decision)

	if approval.Decision != common.PlanAccepted {
		log.Printf("WORKER:   -> REJECTED: exploit execution denied")
		log.Printf("WORKER: ========================================")
		return
	}

	log.Printf("WORKER:   -> APPROVED: creating execution plan...")

	planner := executionplan.NewPlanner(eventStoreAdapter{store: w.eventStore})
	plan, err := planner.CreatePlan(context.Background(), approval)
	if err != nil {
		log.Printf("WORKER:   -> PLANNER ERROR: %v", err)
		log.Printf("WORKER: ========================================")
		return
	}

	log.Printf("WORKER:   -> plan: target=%s payload=%s tool=%s", plan.Target, plan.PayloadType, plan.ToolName)

	planEvent := mission.ExploitExecutionPlannedEvent{
		MissionID:       approval.MissionID,
		ApprovalEventID: approval.EventID(),
		Target:          plan.Target,
		ToolName:        plan.ToolName,
		Recommendation:  plan.Recommendation,
		PlannedAt:       common.Now(),
	}
	w.appendDomainEvent(planEvent)

	log.Printf("WORKER:   -> executing plan via %s plugin...", plan.ToolName)

	result := w.executeExploitPlan(context.Background(), plan)

	resultEvent := mission.ExploitResultEvent{
		MissionID:   approval.MissionID,
		PlanEventID: planEvent.EventID(),
		Target:      plan.Target,
		ToolName:    plan.ToolName,
		Status:      result.Status,
		Evidence:    result.Evidence,
		StartedAt:   result.StartedAt,
		CompletedAt: result.CompletedAt,
	}
	w.appendDomainEvent(resultEvent)

	log.Printf("WORKER:   -> exploit result: status=%s evidence=%d bytes", resultEvent.Status, len(resultEvent.Evidence))
	log.Printf("WORKER: ========================================")
}

type exploitResult struct {
	Status      common.TaskStatus
	Evidence    string
	StartedAt   common.UTCTime
	CompletedAt common.UTCTime
}

func (w *Worker) executeExploitPlan(ctx context.Context, plan *executionplan.ExecutionPlan) exploitResult {
	startedAt := common.Now()

	if plan.PayloadType != "" {
		return w.generatePayload(startedAt, plan)
	}

	tools := w.registry.GetAll()
	var tool execution.ExternalTool
	for _, t := range tools {
		if t.Name() == plan.ToolName {
			tool = t
			break
		}
	}

	if tool == nil {
		return exploitResult{
			Status:      common.TaskFailed,
			Evidence:    fmt.Sprintf("tool %s not found in registry", plan.ToolName),
			StartedAt:   startedAt,
			CompletedAt: common.Now(),
		}
	}

	exploitReq := execution.ToolRequest{
		MissionID: plan.MissionID,
		Target:    plan.Target,
		Options: execution.ToolOptions{
			Timeout: 5 * time.Minute,
		},
		Metadata: map[string]string{
			"source":         "exploit-planner",
			"recommendation": plan.Recommendation,
		},
	}

	result, err := tool.Run(ctx, exploitReq)
	completedAt := common.Now()

	if err != nil {
		return exploitResult{
			Status:      common.TaskFailed,
			Evidence:    fmt.Sprintf("exploit failed: %v", err),
			StartedAt:   startedAt,
			CompletedAt: completedAt,
		}
	}

	evidence := ""
	if result != nil {
		evidence = result.Evidence
	}

	return exploitResult{
		Status:      common.TaskSuccess,
		Evidence:    evidence,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}
}

func (w *Worker) generatePayload(startedAt common.UTCTime, plan *executionplan.ExecutionPlan) exploitResult {
	engine := pkgexploit.NewPayloadEngine()
	payloadType := pkgexploit.PayloadType(plan.PayloadType)

	lhost := envOr("LHOST", "")
	lport := envOr("LPORT", "4444")
	if lhost == "" {
		lhost = "127.0.0.1"
	}

	payload, err := engine.Generate(payloadType, map[string]string{
		"LHOST": lhost,
		"LPORT": lport,
	})
	if err != nil {
		return exploitResult{
			Status:      common.TaskFailed,
			Evidence:    fmt.Sprintf("payload generation failed: %v", err),
			StartedAt:   startedAt,
			CompletedAt: common.Now(),
		}
	}

	log.Printf("WORKER:   -> payload generated: type=%s name=%s lang=%s platform=%s",
		payload.Type, payload.Name, payload.Language, payload.Platform)

	var evidenceBuilder strings.Builder
	fmt.Fprintf(&evidenceBuilder, "[PAYLOAD] %s (%s/%s)\nDescription: %s\n\n--- ORIGINAL ---\n%s",
		payload.Name, payload.Language, payload.Platform,
		payload.Description, payload.Code)

	obfuscator := pkgexploit.NewObfuscator()
	variants := obfuscator.ObfuscateAll(payload.Code)
	if len(variants) > 0 {
		fmt.Fprintf(&evidenceBuilder, "\n\n--- OBFUSCATED VARIANTS (%d) ---", len(variants))
		for _, v := range variants {
			fmt.Fprintf(&evidenceBuilder, "\n[%s]\n%s", v.Technique, v.Code)
		}
		log.Printf("WORKER:   -> %d obfuscated variants generated", len(variants))
	}

	plan.PayloadCode = payload.Code

	return exploitResult{
		Status:      common.TaskSuccess,
		Evidence:    evidenceBuilder.String(),
		StartedAt:   startedAt,
		CompletedAt: common.Now(),
	}
}

func (w *Worker) appendDomainEvent(event interface{}) {
	if w.eventStore == nil {
		return
	}

	domainEvent, ok := event.(interface {
		EventID() string
		EventType() string
		OccurredAt() common.UTCTime
		AggregateID() string
	})
	if !ok {
		return
	}

	payloadBytes, _ := json.Marshal(event)
	record := postgres.EventRecord{
		ID:            domainEvent.EventID(),
		AggregateID:   domainEvent.AggregateID(),
		AggregateType: "mission",
		EventType:     domainEvent.EventType(),
		Payload:       payloadBytes,
		OccurredAt:    domainEvent.OccurredAt().Time,
	}
	if err := w.eventStore.Append(context.Background(), -1, record); err != nil {
		log.Printf("WORKER: WARNING: failed to append %s event: %v", domainEvent.EventType(), err)
	} else {
		log.Printf("WORKER: %s event appended for mission %s", domainEvent.EventType(), domainEvent.AggregateID())
	}
}

func (w *Worker) runAdaptiveLoop(missionID string, target string) {
	log.Printf("WORKER: ========================================")
	log.Printf("WORKER: ADAPTIVE ATTACK LOOP STARTING")
	log.Printf("WORKER:   mission: %s  target: %s", missionID, target)
	log.Printf("WORKER: ========================================")

	cfg := aiProviderConfigFromEnvForAdaptive()
	provider, err := providerfactory.NewProviderFromConfig(cfg)
	if err != nil {
		log.Printf("WORKER: ADAPTIVE: provider setup failed: %v", err)
		return
	}

	llmAdapter := providerfactory.NewLLMClientAdapter(provider)

	config := cognitive.AdaptiveLoopConfig{
		MaxIterations: 20,
		LLMTimeout:    60 * time.Second,
		Logger:        log.Default(),
	}

	loop := cognitive.NewAdaptiveLoop(config, llmAdapter, eventAppender{store: w.eventStore})

	recon := cognitive.ReconSummary{
		TechStack: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := loop.Run(ctx, target, &recon)
	if err != nil {
		log.Printf("WORKER: ADAPTIVE: loop error: %v", err)
		return
	}

	log.Printf("WORKER: ========================================")
	log.Printf("WORKER: ADAPTIVE ATTACK LOOP COMPLETE")
	log.Printf("WORKER:   attempts: %d  successes: %d", result.TotalAttempts, result.SuccessCount)
	log.Printf("WORKER:   reason: %s", result.StopReason)
	log.Printf("WORKER: ========================================")
}

func aiProviderConfigFromEnvForAdaptive() infraai.ProviderConfig {
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		return infraai.ProviderConfig{
			Type:      infraai.ProviderOpenAI,
			APIKey:    key,
			Model:     envOr("DEEPSEEK_MODEL", "deepseek-chat"),
			BaseURL:   envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
			Timeout:   120,
			MaxTokens: 2048,
		}
	}
	return infraai.ProviderConfig{
		Type:    infraai.ProviderOllama,
		Model:   envOr("AI_MODEL", "deepseek-r1:7b"),
		BaseURL: envOr("OLLAMA_BASE_URL", "http://localhost:11434"),
	}
}

type eventAppender struct {
	store *postgres.EventStore
}

func (a eventAppender) Append(ctx context.Context, event interface{}) error {
	domainEvent, ok := event.(interface {
		EventID() string
		EventType() string
		OccurredAt() common.UTCTime
		AggregateID() string
	})
	if !ok {
		return fmt.Errorf("event does not implement domain event contract")
	}

	payloadBytes, _ := json.Marshal(event)
	record := postgres.EventRecord{
		ID:            domainEvent.EventID(),
		AggregateID:   domainEvent.AggregateID(),
		AggregateType: "mission",
		EventType:     domainEvent.EventType(),
		Payload:       payloadBytes,
		OccurredAt:    domainEvent.OccurredAt().Time,
	}
	return a.store.Append(ctx, -1, record)
}

func connectDB() *pgxpool.Pool {
	cfg := postgres.Config{
		Host:     envOr("DB_HOST", "localhost"),
		Port:     5432,
		User:     envOr("DB_USER", "ophidian"),
		Password: envOr("DB_PASSWORD", "ophidian"),
		Database: envOr("DB_NAME", "ophidian"),
		SSLMode:  envOr("DB_SSLMODE", "disable"),
	}
	pool, err := postgres.NewPool(cfg)
	if err != nil {
		log.Fatalf("database connect: %v", err)
	}
	log.Println("WORKER: PostgreSQL connected:", cfg.Host, cfg.Database)
	return pool
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n
		}
	}
	return def
}

func startAIEventSubscriber(ctx context.Context, eventStore *postgres.EventStore) {
	log.Println("AI Subscriber starting...")
	cfg, ok := aiProviderConfigFromEnv()
	if !ok {
		log.Printf("AI-SUBSCRIBER/WARN: disabled: DEEPSEEK_API_KEY is not set")
		return
	}
	provider, err := providerfactory.NewProviderFromConfig(cfg)
	if err != nil {
		log.Printf("AI SUBSCRIBER ERROR: provider setup failed: %v", err)
		return
	}
	log.Printf("AI-SUBSCRIBER: provider=%s model=%s base_url=%s", cfg.Type, cfg.Model, cfg.BaseURL)

	stream := eventStreamAdapter{store: eventStore}
	llm := providerfactory.NewLLMClientAdapter(provider)
	subscriber := appai.NewEventSubscriber(stream, llm, 5*time.Second, log.Default())
	subscriber.Run(ctx)
}

func aiProviderConfigFromEnv() (infraai.ProviderConfig, bool) {
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		return infraai.ProviderConfig{
			Type:      infraai.ProviderOpenAI,
			APIKey:    key,
			Model:     envOr("DEEPSEEK_MODEL", "deepseek-chat"),
			BaseURL:   envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
			Timeout:   60,
			MaxTokens: 1024,
		}, true
	}

	return infraai.ProviderConfig{}, false
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("WORKER: .env not loaded from %s: %v", path, err)
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, value)
	}
	log.Printf("WORKER: .env loaded from %s", path)
}

type eventStreamAdapter struct {
	store *postgres.EventStore
}

type eventStoreAdapter struct {
	store *postgres.EventStore
}

func (a eventStoreAdapter) LoadAllEvents(ctx context.Context, from, to time.Time) ([]executionplan.StoredEvent, error) {
	records, err := a.store.LoadAllEvents(ctx, from, to)
	if err != nil {
		return nil, err
	}
	events := make([]executionplan.StoredEvent, len(records))
	for i, r := range records {
		events[i] = executionplan.StoredEvent{
			ID:          r.ID,
			AggregateID: r.AggregateID,
			EventType:   r.EventType,
			Payload:     r.Payload,
		}
	}
	return events, nil
}

func (a eventStreamAdapter) LoadEventsSince(ctx context.Context, since time.Time) ([]appai.StoredEvent, error) {
	records, err := a.store.LoadAllEvents(ctx, since, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	events := make([]appai.StoredEvent, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
		events = append(events, appai.StoredEvent{
			ID:          record.ID,
			AggregateID: record.AggregateID,
			EventType:   record.EventType,
			Payload:     record.Payload,
			OccurredAt:  record.OccurredAt,
		})
	}
	return events, nil
}

func (a eventStreamAdapter) Append(ctx context.Context, event interface{}) error {
	domainEvent, ok := event.(interface {
		EventID() string
		EventType() string
		OccurredAt() common.UTCTime
		AggregateID() string
	})
	if !ok {
		return fmt.Errorf("event does not implement domain event contract")
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	record := postgres.EventRecord{
		ID:            domainEvent.EventID(),
		AggregateID:   domainEvent.AggregateID(),
		AggregateType: "mission",
		EventType:     domainEvent.EventType(),
		Payload:       payload,
		OccurredAt:    domainEvent.OccurredAt().Time,
	}
	return a.store.Append(ctx, -1, record)
}

type stdLogger struct{}

func (l stdLogger) Info(msg string, kv ...interface{})  { log.Printf("QUEUE: %s %v", msg, kv) }
func (l stdLogger) Warn(msg string, kv ...interface{})  { log.Printf("QUEUE/WARN: %s %v", msg, kv) }
func (l stdLogger) Error(msg string, kv ...interface{}) { log.Printf("QUEUE/ERROR: %s %v", msg, kv) }
