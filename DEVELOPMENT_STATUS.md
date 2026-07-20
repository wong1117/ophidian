# DEVELOPMENT_STATUS.md — Ophidian Development Status

## Phase History

### Phase 1: Domain Foundation
- Mission aggregate with lifecycle states (DRAFT, ACTIVE, PAUSED, COMPLETED, ABORTED, FAILED)
- Event types: MissionStarted, MissionStateChanged, PhaseTransitioned, TaskDispatched, TaskCompleted
- RoE validation (max targets, time windows, excluded nets)
- Aggregate invariants (status transitions, phase transitions)
- 12 domain packages: mission, attackplan, target, finding, session, report, policy, feature, graph, rbac, tenant

### Phase 2: Application Layer
- CreateMissionUseCase with full lifecycle (validate RoE, create aggregate, start, persist)
- OrchestrateMissionUseCase with lifecycle actions (PLAN, READY, RUN, COMPLETE, FAIL)
- GeneratePlanUseCase with AI recommendations and attack path strategies
- Exploit orchestrator, report generator, cleanup services
- Cross-cutting: audit logging, backup verification, opsec LOTL biasing

### Phase 3: Infrastructure — Persistence
- PostgreSQL connection pool via pgxpool
- MissionRepository (Save, FindByID, FindAll, Update, Delete)
- Task persistence (SaveTask, FindTaskByID, FindTasksByMission, UpdateTask)
- EventStore (Append, LoadStream, Migrate)
- Redis session store and pubsub
- Configuration via YAML files

### Phase 4: Infrastructure — HTTP & Web
- Echo v4 HTTP server with middleware (logging, recovery, CORS)
- REST handlers: Mission (CRUD + lifecycle), Recon (passive/active), Exploit, AI, Report
- pprof endpoints for performance debugging
- Health endpoint with uptime tracking

### Phase 5: Event Sourcing Bridge
- EventDispatcher interface defined as application port
- HTTPEventDispatcher adapter (POST events to worker HTTP endpoint)
- Server dispatches events after EventStore.Append
- All CreateMission and OrchestrateMission use cases wired with dispatcher

### Phase 6: Worker — Event Consumer
- Worker binary with HTTP server (:9090) for event reception
- In-memory PriorityQueue with heap-based priority ordering
- Worker event loop polling queue every 1 second
- Event handlers: MissionStarted, MissionStateChanged, PhaseTransitioned, TaskDispatched
- Graceful shutdown on SIGINT/SIGTERM

### Phase 7: Worker — Target Data Flow
- Worker connects to PostgreSQL for aggregate state loading
- MissionStarted handler loads full mission from DB via MissionRepository.FindByID
- Target domains and IPs are extracted from loaded mission state
- Recon handler logs target details before phase execution
- Event Sourcing pattern: trigger event (metadata only) + aggregate replay (full state)

## Technical Achievements

- PostgreSQL wired with pgxpool: `postgres://ophidian:ophidian@localhost:5432/ophidian`
- 4 tables: `missions`, `mission_tasks`, `events`, `aggregate_snapshots`
- HTTP Bridge verified: server → dispatcher → worker event reception
- Worker successfully loads aggregate state from database
- Domain invariants enforced: cannot restart ACTIVE mission (422), RoE validation (403)
- UTCTime implements driver.Valuer and sql.Scanner for pgx compatibility
- Both binaries (`ophidian-server`, `ophidian-worker`) build and vet clean
- 282 total `.go` source files

## Current Task: MVAC Step 2 of 4

Building the Minimum Viable Attack Chain (MVAC) in 4 steps:

| Step | Status | Description |
|------|--------|-------------|
| 1 | Done | ReconCompletedEvent domain definition (`internal/domain/mission/recon_events.go`) |
| 2 | Done | NmapRunner infrastructure adapter (`internal/infrastructure/runner/nmap_runner.go`) |
| 3 | Pending | Wire NmapRunner into Worker recon handler |
| 4 | Pending | End-to-end test: curl POST mission → Worker runs Nmap → ReconCompletedEvent stored |

### Step 1 — ReconCompletedEvent (Done)
- File: `internal/domain/mission/recon_events.go`
- Implements `mission.DomainEvent` interface
- Fields: MissionID, Target, RawOutput, Status (common.TaskStatus), StartedAt, CompletedAt

### Step 2 — NmapRunner (Done)
- File: `internal/infrastructure/runner/nmap_runner.go`
- Interface: `Runner` with `Run(ctx context.Context, target string) (string, error)`
- Implementation: `NmapRunner` using `exec.CommandContext`
- Command: `nmap -sV -Pn --top-ports 100 <target>`
- Validates: empty target, missing binary, context cancellation

## Known Issues

1. **Docker Hub unreachable** — cannot pull NATS server or Redis images. Workaround: HTTP-based dispatcher bridge instead of NATS messaging
2. **Nmap stubbed** — NmapRunner exists but is not yet wired into worker's MissionStarted handler
3. **Multiple recon per mission** — ReconCompletedEvent uses MissionID as EventID; multiple recon runs on same mission would conflict in events table
4. **Pre-existing build errors** — Several packages (saga, ai, messaging/nats, crypto, network) have compilation errors unrelated to current work
5. **TUI freeze** — Terminal UI may hang on long-running operations

## Next Phase (Phase 8)

- Wire NmapRunner into Worker's handleMissionStarted handler
- Worker executes real Nmap scan and produces ReconCompletedEvent
- Dispatch ReconCompletedEvent back to server via HTTP bridge
- Persist ReconCompletedEvent in EventStore
- Display recon results in server logs
- Add scan scheduling and rate limiting
- Integrate more recon tools (Nikto, Gobuster, WhatWeb)
