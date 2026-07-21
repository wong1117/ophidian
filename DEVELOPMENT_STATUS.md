# DEVELOPMENT_STATUS.md — Ophidian Development Status

This file tracks implementation status synchronized with the ERA structure in ROADMAP.md.

---

## Era 1: The Foundation — COMPLETED

Domain layer, Event Store, aggregates, CQRS separation.

| Phase | Description | Status |
|-------|-------------|--------|
| 1.1-1.5 | Core Setup: project scaffold, PostgreSQL, Echo HTTP, domain entities, CQRS | Done |
| 1.6-1.8 | Event Sourcing Core: EventStore, Mission aggregate, snapshots, domain events | Done |

Key deliverables: 12 domain packages, `common.ID`/`common.UTCTime`, `mission.DomainEvent` interface, `postgres.EventStore` (Append, LoadStream, Migrate), aggregate invariants, RoE validation.

---

## Era 2: The Infrastructure & Planes — COMPLETED

Three-plane components, CLI, AI interface, observability.

| Phase | Description | Status |
|-------|-------------|--------|
| 2.1 | Control Plane: Mission HTTP handlers, CLI skeleton, TUI skeleton | Done |
| 2.2 | Execution Plane: Worker skeleton, job queue mechanism (HTTP bridge) | Done |
| 2.3 | AI Plane: LLM Client interface, Vector DB integration, prompt templates | Done |
| 2.4 | Observability: OpenTelemetry, Prometheus metrics, pprof | Done |

Key deliverables: `Echo v4` REST API (`:8443`), `Cobra` CLI, `Bubble Tea` TUI, worker binary (`:9090`), LLM provider adapters, `chromem-go` RAG.

---

## Era 3: Engineering Excellence — COMPLETED

Performance, reliability, governance, supply chain.

| Phase | Description | Status |
|-------|-------------|--------|
| 3.1 | Performance: benchmarking, caching (Ristretto/Redis), profiling | Done |
| 3.2 | Reliability: graceful shutdown, fault injection, chaos testing | Done |
| 3.3 | Governance: architecture linters, dependency enforcement, cycle detection | Done |
| 3.4 | Supply Chain: vulnerability scanning, SBOM generation | Done |

Key deliverables: `queue.PriorityQueue` benchmarking, graceful shutdown (worker + server), `.golangci.yml` with 30+ linters, `cmd/archlint`, SBOM in CI.

---

## Era 4: The Operational MVP — COMPLETED

Prove the architecture works with a real end-to-end attack cycle. The MVAC (Minimum Viable Attack Cycle) is complete.

### Phase 4.1: MVAC (Minimum Viable Attack Cycle) — DONE

| Step | Description | Status |
|------|-------------|--------|
| 1 | Define `ReconCompletedEvent` domain event | Done |
| 2 | Build `NmapRunner` infrastructure adapter | Done |
| 3 | Wire NmapRunner into Worker handler | Done |
| 4 | Append ReconCompletedEvent to EventStore | Done |
| 5 | Verify persistence in PostgreSQL via curl | Done |

**Verified flow:**
```
curl POST /missions {ips:["127.0.0.1"]}
  → server: missionRepo.Save + eventStore.Append(MissionStarted) + dispatcher.Dispatch
    → worker: receives MissionStarted → loads mission from DB
      → nmap -sV -Pn --top-ports 100 --host-timeout 15s 127.0.0.1
      → ReconCompletedEvent {Status:SUCCESS, 351 bytes}
      → eventStore.Append(ReconCompletedEvent) → persisted in PostgreSQL
```

Both `MissionStarted` and `ReconCompleted` events confirmed in `events` table with proper aggregate IDs.

### Remaining Era 4 Phases (pending)

| Phase | Description |
|-------|-------------|
| 4.2 | AI Feedback Loop: subscribe to ReconCompleted, query LLM, generate ExploitSuggested |
| 4.3 | Human-In-The-Loop: fix TUI freeze, display AI recommendations, Y/n approve/reject |
| 4.4 | Execution Trigger: worker listens for Approval events, runs exploit |
| 4.5 | Live Dashboard: bidirectional dispatch, TUI real-time updates |

---

## Era 5: Arsenal Expansion & Real Offense — CURRENT ERA

Replace stubs with real offensive tools. Expand reconnaissance capabilities.

| Phase | Description | Status |
|-------|-------------|--------|
| 5.1 | Web Exploitation Engine: chromedp, HTTP forgery, session handling | Pending |
| 5.2 | Advanced Reconnaissance: Subfinder, Amass, Feroxbuster, JS parsing, Nikto, WhatWeb, Gobuster, parallel scanning, rate limiting | Pending |
| 5.3 | Exploit Acquisition: PoC auto-fetcher, N-Day cache, payload templates | Pending |
| 5.4 | Evasion & Stealth: payload obfuscation, LoLBins, fileless execution | Pending |

---

## Era 6: Exoskeleton Intelligence — Pending

AI-driven cross-target learning and autonomous scoping.

---

## Era 7: Infrastructure Maturity — Pending

Replace temporary bridges with enterprise-grade infrastructure: NATS JetStream, gRPC, circuit breakers, retry/backoff. Also: fix pre-existing build errors in saga, ai, messaging/nats, crypto, network packages.

---

## Era 8: Reporting & Tradecraft — Pending

Automated kill chain reporting, executive summaries, PoC generation, OPSEC cleanup, self-destruct mode.

---

## Technical Overview

| Metric | Value |
|--------|-------|
| Go version | 1.22 |
| Database | PostgreSQL 16 (Docker, `ophidian:ophidian@localhost:5432/ophidian`) |
| Tables | `missions`, `mission_tasks`, `events`, `aggregate_snapshots` |
| Binaries | `build/ophidian-server` (`:8443`), `build/ophidian-worker` (`:9090`) |
| Source files | 282 `.go` files |
| Domain tests | Covers `mission`, `policy`, `finding` |
| Build status | `go build ./cmd/...` passes clean |

## Known Issues

1. **Docker Hub unreachable** — cannot pull NATS or Redis images. Workaround: HTTP-based dispatcher bridge.
2. **Nmap `--host-timeout` aggressive** — 15s timeout may cut off service detection on slow targets.
3. **Pre-existing APPLICATION_PURITY warnings** — 4 use cases import `internal/interfaces/dto` (warnings only, archlint passes).
4. **TUI freeze** — Bubble Tea input blocking. Tracked in Era 4 Phase 4.3.
5. **UTCTime Scan interface** — Custom type works with pgx via `driver.Valuer` + custom `Scan`; not implementing standard `sql.Scanner`.

## Startup Order

1. PostgreSQL: `docker start ophidian-pg`
2. Worker: `./build/ophidian-worker`
3. Server: `./build/ophidian-server`
