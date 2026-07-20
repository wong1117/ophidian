# AGENTS.md — Ophidian Development Rules

## Stack

Go 1.22, Clean Architecture, Domain-Driven Design, CQRS, Event Sourcing, Hexagonal (Ports & Adapters). PostgreSQL via pgxpool. HTTP via Echo v4.

## Three-Plane Architecture (Non-Negotiable)

| Plane | Responsibility | Must NOT |
|-------|---------------|----------|
| Control Plane | REST API, orchestration, event dispatch | Execute exploits, make AI decisions |
| Execution Plane (Worker) | Recon, exploit, post-exploit, report | Decide mission strategy |
| AI Plane | Recommendations, plan generation | Issue commands autonomously |

AI is always an Advisor, never a Commander. The AI Plane proposes; the Control Plane decides; the Execution Plane executes.

## Domain Events & Event Sourcing

Every state change MUST produce a domain event. All events implement `DomainEvent` interface: `EventID()`, `EventType()`, `OccurredAt()`, `AggregateID()`, `Version()`.

Events are persisted to PostgreSQL `events` table. The event store is the single source of truth. Aggregate state is reconstructed by replaying events.

## Code Organization

| Directory | Purpose |
|-----------|---------|
| `cmd/` | Binary entry points (server, worker, cli) |
| `internal/domain/` | Aggregates, entities, value objects, domain events, repository interfaces |
| `internal/application/` | Use cases, application services, sagas |
| `internal/infrastructure/` | Adapters: persistence, messaging, runner, web, queue |
| `internal/interfaces/` | DTOs, cross-plane event mappers |

## Dependency Rule

Domain depends on nothing. Application depends on domain. Infrastructure depends on application + domain. Interfaces bridge planes.

## Error Handling

- Domain errors use sentinel errors in `internal/domain/common/errors.go`
- Infrastructure wraps domain errors: `fmt.Errorf("save mission: %w", err)`
- HTTP handlers return domain errors directly; Echo error handler maps them to status codes
- Worker logs warnings on non-fatal errors, retries with backoff on transient failures

## Testing

- Domain: unit tests for invariants (`go test ./internal/domain/...`)
- Infrastructure: integration tests for adapters (PostgreSQL, runner)
- Worker: verify event consumption via log output
- Server: verify API contracts via `curl` against running instance

## Workflow

1. Read `DEVELOPMENT_STATUS.md` for current phase
2. Read `ARCHITECTURE.md` for code location
3. Run `go build ./cmd/...` and `go vet ./cmd/...` before committing
4. Run `go test ./internal/domain/...` for domain invariant verification
