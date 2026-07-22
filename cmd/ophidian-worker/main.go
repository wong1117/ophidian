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
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	appai "github.com/ophidian/ophidian/internal/application/ai"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
	infraai "github.com/ophidian/ophidian/internal/infrastructure/ai"
	"github.com/ophidian/ophidian/internal/infrastructure/ai/providerfactory"
	"github.com/ophidian/ophidian/internal/infrastructure/dispatcher"
	"github.com/ophidian/ophidian/internal/infrastructure/persistence/postgres"
	"github.com/ophidian/ophidian/internal/infrastructure/queue"
	"github.com/ophidian/ophidian/internal/infrastructure/runner"
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
	q := queue.NewPriorityQueue(nil, queue.WithQueueLogger(stdLogger{}))
	worker := NewWorker(q, missionRepo, nmapRunner, eventStore)

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
	runner      runner.Runner
	eventStore  *postgres.EventStore
}

func NewWorker(q *queue.PriorityQueue, repo *postgres.MissionRepository, r runner.Runner, es *postgres.EventStore) *Worker {
	return &Worker{q: q, missionRepo: repo, runner: r, eventStore: es}
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

	for _, target := range targets {
		w.runReconForTarget(common.ID(payload.MissionID), target)
	}
}

func (w *Worker) runReconForTarget(missionID common.ID, target string) {
	startedAt := common.Now()
	log.Printf("WORKER: -> scanning: %s", target)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rawOutput, scanErr := w.runner.Run(ctx, target)
	completedAt := common.Now()

	var status common.TaskStatus
	if scanErr != nil {
		status = common.TaskFailed
		log.Printf("WORKER: -> scan FAILED for %s: %v", target, scanErr)
	} else {
		status = common.TaskSuccess
		log.Printf("WORKER: -> scan COMPLETE for %s (%d bytes)", target, len(rawOutput))
	}

	event := mission.ReconCompletedEvent{
		MissionID:   missionID,
		Target:      target,
		RawOutput:   rawOutput,
		Status:      status,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}

	log.Printf("WORKER: \u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014\u2014")
	log.Printf("WORKER: ReconCompletedEvent")
	log.Printf("WORKER:   mission_id:  %s", event.MissionID)
	log.Printf("WORKER:   target:      %s", event.Target)
	log.Printf("WORKER:   status:      %s", event.Status)
	log.Printf("WORKER:   output_len:  %d", len(event.RawOutput))
	log.Printf("WORKER:   started:     %s", event.StartedAt.Time.Format(time.RFC3339))
	log.Printf("WORKER:   completed:   %s", event.CompletedAt.Time.Format(time.RFC3339))
	if event.RawOutput != "" {
		log.Printf("WORKER:   output_preview: %.200s", event.RawOutput)
	}
	log.Printf("WORKER: ——————————————————")

	if w.eventStore != nil {
		payloadBytes, _ := json.Marshal(event)
		record := postgres.EventRecord{
			ID:            event.EventID(),
			AggregateID:   event.AggregateID(),
			AggregateType: "mission",
			EventType:     event.EventType(),
			Payload:       payloadBytes,
			OccurredAt:    event.CompletedAt.Time,
		}
		if err := w.eventStore.Append(context.Background(), -1, record); err != nil {
			log.Printf("WORKER: WARNING: failed to append event to store: %v", err)
		} else {
			log.Printf("WORKER: Recon event appended to store for mission %s", event.MissionID)
		}
	}
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
