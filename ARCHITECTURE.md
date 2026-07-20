# ARCHITECTURE.md — Ophidian System Architecture

## Directory Map

```
cmd/
  ophidian-server/       Control Plane binary (REST API :8443)
  ophidian-worker/       Execution Plane binary (event consumer :9090)
  ophidian-cli/          CLI tool for operator interaction

internal/
  domain/                Domain layer — zero dependencies
    common/              Shared types: ID, UTCTime, Severity, Phase, TaskStatus
    mission/             Mission aggregate, lifecycle events, RoE rules
    attackplan/          Attack plan aggregate and events
    target/              Target aggregate, service detection events
    finding/             Security finding aggregate
    session/             Exploit session aggregate
    report/              Report aggregate
    policy/              Policy aggregate
    feature/             Feature flag domain
    graph/               Attack graph domain
    rbac/                Role-based access control domain
    tenant/              Multi-tenancy domain

  application/           Application layer — use cases, sagas
    controlplane/        CreateMission, OrchestrateMission, EventDispatcher, EventStore
    aiplane/             GeneratePlan, recommendation engines
    executionplane/      Exploit, recon, report use cases
    memory/              Context memory for AI
    cognitive/           TTP adaptation, learning
    opsec/               LOTL biasing, stealth tactics
    saga/                Long-running transaction sagas
    cobra/               Attack path graph algorithms

  infrastructure/        Infrastructure layer — adapters
    persistence/         PostgreSQL (pgxpool), Redis
      postgres/          MissionRepository, EventStore, connection pool
      redis/             Cache, session store, pubsub
    messaging/           NATS publisher/subscriber (available, not wired)
    dispatcher/          HTTPEventDispatcher (server-to-worker bridge)
    queue/               In-memory priority job queue
    runner/              External tool runners (NmapRunner)
    web/                 Echo HTTP server, route registration, handlers
    config/              YAML config loader
    telemetry/           Metrics, logging
    crypto/              TLS, encryption
    secrets/             Secret management
    ai/                  AI provider adapters
    arsenal/             Exploit tool integrations
    workflow/            DAG workflow engine
    scheduler/           Task scheduling

  interfaces/            Cross-plane bridges
    dto/                 Request/response data transfer objects
    event_mappers.go     Cross-plane event transformation
    control_to_exec.go   Control Plane to Execution Plane mapping
    control_to_ai.go     Control Plane to AI Plane mapping
    ai_to_exec.go        AI Plane to Execution Plane mapping
```

## Design Patterns

| Pattern | Usage | Location |
|---------|-------|----------|
| Repository | Domain persistence interface; PostgreSQL adapter implements it | `internal/domain/*/repository.go`, `internal/infrastructure/persistence/postgres/` |
| Event Sourcing | All state changes produce events; EventStore as source of truth | `internal/domain/*/events.go`, `internal/infrastructure/persistence/postgres/event_store.go` |
| Strategy | AI provider selection, TTP adaptation | `internal/infrastructure/ai/`, `internal/application/cognitive/` |
| Saga | Long-running workflows (attack campaigns) | `internal/application/saga/` |
| Circuit Breaker | External tool execution resilience | `internal/infrastructure/worker/` |
| Ports & Adapters | Every plane communicates through well-defined interfaces | `internal/application/*/` interfaces, `internal/infrastructure/*/` adapters |

## Plane Locations

### Control Plane
- **Binary:** `cmd/ophidian-server/main.go`
- **Use cases:** `internal/application/controlplane/`
- **HTTP handlers:** `internal/infrastructure/web/handlers/`
- **Event dispatch:** `internal/infrastructure/dispatcher/http.go`
- **Config:** `configs/control-plane.yaml`

### Execution Plane (Worker)
- **Binary:** `cmd/ophidian-worker/main.go`
- **Use cases:** `internal/application/executionplane/`
- **Queue:** `internal/infrastructure/queue/queue.go`
- **Tool runners:** `internal/infrastructure/runner/`
- **Config:** `configs/execution-plane.yaml`

### AI Plane
- **Use cases:** `internal/application/aiplane/`
- **AI providers:** `internal/infrastructure/ai/`
- **Config:** `configs/ai-plane.yaml`

### Event Store
- **Interface:** `internal/application/controlplane/event_store.go` (port)
- **PostgreSQL adapter:** `internal/infrastructure/persistence/postgres/event_store.go`
- **Tables:** `events`, `aggregate_snapshots`

## Message Flow

```
POST /api/v1/missions
  → CreateMissionUseCase
    → missionRepo.Save (PostgreSQL missions table)
    → eventStore.Append (PostgreSQL events table)
    → dispatcher.Dispatch (HTTP POST worker:9090/events)
      → Worker /events handler
        → PriorityQueue.Enqueue
          → Worker event loop dequeue
            → handler (MissionStarted → FindByID → recon)
```

## Configuration

| File | Purpose |
|------|---------|
| `configs/ophidian.yaml` | Main configuration (database, NATS, Redis, TLS) |
| `configs/control-plane.yaml` | Control Plane specific settings |
| `configs/execution-plane.yaml` | Worker specific settings |
| `configs/ai-plane.yaml` | AI provider settings |
| `configs/roe.example.yaml` | Rules of Engagement templates |
