# Ophidian Developer Onboarding Guide

## Prerequisites

- **Go 1.22+** ([download](https://go.dev/dl/))
- **Docker** (for PostgreSQL, Redis, NATS)
- **Make** (build automation)
- **golangci-lint** (static analysis)

## Quick Start

```bash
git clone https://github.com/ophidian/ophidian.git
cd ophidian
make dev-setup          # Install dependencies, create local config
make build              # Build all binaries
make run-server         # Start the API server
```

## Project Structure

```
ophidian/
├── cmd/                    # Entry points
│   ├── ophidian-server/    # HTTP API server
│   ├── ophidian-cli/       # CLI tool
│   ├── ophidian-agent/     # Deployment agent
│   └── ophidian-worker/    # Background worker
├── internal/
│   ├── domain/             # Domain entities, value objects, repositories (interfaces)
│   │   ├── common/         # Shared types (ID, UTCTime, errors)
│   │   ├── mission/        # Mission aggregate
│   │   ├── attackplan/     # Attack plan aggregate
│   │   ├── target/         # Target entity
│   │   ├── finding/        # Security finding entity
│   │   └── session/        # Session entity
│   ├── application/        # Use cases / service layer
│   │   ├── controlplane/   # Mission lifecycle orchestration
│   │   ├── aiplane/        # AI-driven planning
│   │   ├── executionplane/ # Recon, exploit, post-exploit, reporting
│   │   ├── policy/         # Policy evaluation engine
│   │   └── recommendation/ # Scoring and ranking engine
│   ├── infrastructure/     # Adapters (database, messaging, AI, config)
│   │   ├── persistence/postgres/  # PostgreSQL repositories + EventStore
│   │   ├── persistence/redis/     # Redis cache + session store
│   │   ├── messaging/             # NATS + RabbitMQ
│   │   ├── ai/                    # AI providers (OpenAI, Anthropic, Ollama)
│   │   ├── workflow/              # DAG workflow engine
│   │   ├── queue/                 # Priority job queue
│   │   ├── scheduler/             # Cron scheduler
│   │   ├── worker/                # Distributed worker pool
│   │   ├── secrets/               # Encrypted secret management
│   │   ├── config/                # Configuration loader
│   │   ├── ha/                    # Health checks, leader election
│   │   └── web/                   # HTTP handlers + middleware
│   └── interfaces/         # DTOs, event mappers, inter-plane contracts
├── pkg/                    # Shared libraries
│   ├── plugins/            # Plugin SDK
│   └── protocols/          # HTTP client
├── deploy/                 # Deployment manifests
│   └── helm/ophidian/      # Helm chart
├── docs/                   # Documentation
├── examples/               # Runnable examples
├── test/                   # Integration + e2e tests
└── configs/                # Configuration files
```

## Architecture Principles

1. **Clean Architecture**: Dependency flows inward. Application → Domain → (no outward deps)
2. **DDD**: Aggregates, value objects, repositories, domain events
3. **CQRS**: Read models separate from write models (dashboard, audit)
4. **Event Sourcing**: EventStore with versioned optimistic concurrency
5. **Hexagonal Ports & Adapters**: Domain defines ports (interfaces), Infrastructure implements them

## Common Commands

```bash
make build             # Build all binaries
make test              # Run all unit tests
make test-race         # Run all tests with race detector
make test-coverage     # Generate HTML coverage report
make lint              # Run static analysis
make fuzz              # Run fuzz tests (30 seconds)
make dev-setup         # Set up local dev environment
make dev-reset         # Reset caches and modules
make run-server        # Start API server (auto-reload with air if installed)
make scaffold NAME=my-service TEMPLATE=service  # Generate new service
make docs              # Generate OpenAPI/Swagger docs

# Quality pipeline
make quality           # lint + race + coverage
make check             # build + lint + race (CI)
```

## Adding a New Feature

1. **Domain**: Define entities and repository interface in `internal/domain/<name>/`
2. **Application**: Implement service/use case in `internal/application/<name>/`
3. **Infrastructure**: Implement repository in `internal/infrastructure/persistence/postgres/`
4. **Tests**: Write unit tests with testify/assert and testify/mock
5. **DTOs**: Add response types in `internal/interfaces/dto/` if exposing via API

## Service Template

Use the scaffolding command:
```bash
make scaffold NAME=user-service TEMPLATE=service
```

This generates:
- `internal/application/user-service/user_service.go` — with Execute method
- `internal/application/user-service/user_service_test.go` — with success + error tests

## Debugging

```bash
# Run with race detector
go test -race ./internal/application/... -run TestName -count=1

# Profile CPU
go test -bench=. -cpuprofile=cpu.prof ./internal/infrastructure/queue/...
go tool pprof -http=:8080 cpu.prof

# Debug specific package
go test -v -count=1 -run TestFeature ./internal/application/feature/...
```
