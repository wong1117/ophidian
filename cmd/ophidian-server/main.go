package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/application/aiplane"
	application "github.com/ophidian/ophidian/internal/application/controlplane"
	"github.com/ophidian/ophidian/internal/infrastructure/dispatcher"
	"github.com/ophidian/ophidian/internal/infrastructure/persistence/postgres"
	"github.com/ophidian/ophidian/internal/infrastructure/web"
	"github.com/ophidian/ophidian/internal/infrastructure/web/handlers"
)

func main() {
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	pool := connectDB()
	defer pool.Close()

	eventStore := postgres.NewEventStore(pool)
	eventStore.Migrate(context.Background())

	missionRepo := postgres.NewMissionRepository(pool)

	eventDispatcher := dispatcher.NewHTTPEventDispatcher(
		envOr("WORKER_URL", "http://localhost:9090"),
	)

	createUC := application.NewCreateMissionUseCase(missionRepo, wrapEventStore(eventStore), eventDispatcher)
	orchestrateUC := application.NewOrchestrateMissionUseCase(missionRepo, wrapEventStore(eventStore), eventDispatcher)

	planUC := aiplane.NewGeneratePlanUseCase(nil, nil, missionRepo, nil, nil, wrapEventStore(eventStore))

	server := web.NewServer(web.ServerDeps{
		HealthHandler:  handlers.NewHealthHandler(),
		MissionHandler: handlers.NewMissionHandler(createUC, orchestrateUC, missionRepo),
		ReconHandler:   handlers.NewReconHandler(nil),
		ExploitHandler: handlers.NewExploitHandler(nil, nil, nil),
		AIHandler:      handlers.NewAIHandler(planUC, nil),
		ReportHandler:  handlers.NewReportHandler(nil),
	})

	registerPprof(server.Echo)

	log.Println("Ophidian server (PostgreSQL) starting on :8443")
	if err := server.Start(":8443"); err != nil {
		log.Fatal(err)
	}
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
	log.Println("PostgreSQL connected:", cfg.Host, cfg.Database)
	return pool
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// wrapEventStore adapts postgres.EventStore to controlplane.EventStore interface
func wrapEventStore(es *postgres.EventStore) *eventStoreAdapter {
	return &eventStoreAdapter{es: es}
}

type eventStoreAdapter struct {
	es *postgres.EventStore
}

func (a *eventStoreAdapter) Append(ctx context.Context, event interface{}) error {
	evt, ok := event.(interface {
		EventID() string
		EventType() string
		AggregateID() string
	})
	if !ok {
		return a.es.Append(ctx, -1, postgres.EventRecord{
			ID:            "evt-unknown",
			AggregateID:   "unknown",
			AggregateType: "unknown",
			EventType:     "UnknownEvent",
			Payload:       mustMarshal(event),
			OccurredAt:    time.Now(),
		})
	}

	return a.es.Append(ctx, -1, postgres.EventRecord{
		ID:            evt.EventID(),
		AggregateID:   evt.AggregateID(),
		AggregateType: "mission",
		EventType:     evt.EventType(),
		Payload:       mustMarshal(event),
		OccurredAt:    time.Now(),
	})
}

func (a *eventStoreAdapter) Replay(ctx context.Context, aggregateID string) ([]interface{}, error) {
	return a.es.Replay(ctx, aggregateID)
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func registerPprof(e *echo.Echo) {
	e.GET("/debug/pprof/", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/heap", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/goroutine", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/block", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/mutex", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	e.GET("/debug/pprof/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	e.GET("/debug/pprof/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	e.GET("/debug/pprof/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
}
