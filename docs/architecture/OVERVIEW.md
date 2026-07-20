# Ophidian Architecture

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Layer (Echo)                           │
│  Handlers → DTOs → Middleware (Auth, Logging, Metrics, Tenant)   │
├─────────────────────────────────────────────────────────────────┤
│                      Application Layer                            │
│  Use Cases / Services (stateless orchestration)                   │
│  ┌───────────┐ ┌──────────┐ ┌──────────────┐                    │
│  │Control Plane│ │ AI Plane │ │Execution Plane│                    │
│  │  Mission    │ │  Plan    │ │  Recon        │                    │
│  │  Policy     │ │  Audit   │ │  Exploit      │                    │
│  │  RBAC       │ │  Graph   │ │  Report       │                    │
│  │  Dashboard  │ │  Memory  │ │  Cleanup      │                    │
│  └───────────┘ └──────────┘ └──────────────┘                    │
├─────────────────────────────────────────────────────────────────┤
│                       Domain Layer                                 │
│  Entities · Value Objects · Repositories (interfaces)             │
│  ┌─────────┐ ┌───────────┐ ┌──────────┐ ┌────────┐             │
│  │ Mission │ │ AttackPlan │ │  Target  │ │Finding │             │
│  │ Session │ │  Graph     │ │  Policy  │ │Tenant  │             │
│  └─────────┘ └───────────┘ └──────────┘ └────────┘             │
├─────────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                            │
│  Adapters implementing domain interfaces                          │
│  ┌──────────┐ ┌───────┐ ┌──────────┐ ┌────────┐ ┌──────────┐ │
│  │PostgreSQL│ │ Redis │ │  NATS    │ │   AI   │ │ Workflow │ │
│  │EventStore│ │ Cache │ │ RabbitMQ │ │Providers│ │  Engine  │ │
│  │  Queue   │ │  HA   │ │Scheduler │ │Config  │ │ Secrets  │ │
│  └──────────┘ └───────┘ └──────────┘ └────────┘ └──────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow

### Write Path (CQRS + Event Sourcing)
```
HTTP Request → Handler → Use Case → Aggregate → Repository.Save()
                                          ↓
                                    EventStore.Append(event)
                                          ↓
                                    Read Model (Projections)
```

### Read Path
```
HTTP Request → Handler → Dashboard/Audit Service → Read Repository → DTO → JSON
```

## Key Design Patterns

| Pattern | Where | Why |
|---------|-------|-----|
| **Aggregate Root** | MissionAggregate, AttackPlanAggregate | Ensures invariants, single write point |
| **Event Sourcing** | EventStore (postgres) | Audit trail, replay, temporal queries |
| **Repository** | Domain interfaces → Postgres/Redis impls | Decouples domain from storage |
| **Strategy** | ExploitExecutor, AI Provider | Runtime-swappable implementations |
| **Factory** | PluginManager, ProviderFactory | Object creation abstraction |
| **Observer** | Config watchers, Plugin events | Loose coupling for change notification |
| **Decorator** | CacheMetrics, AuditLogger | Add behavior without modifying core |
| **Singleton** | ConfigService, SecretManager | Centralized state management |
| **Dependency Injection** | Constructor injection throughout | Testability, loose coupling |

## Dependency Direction

```
Interfaces ← Application → Domain ← Domain
     ↓            ↓            ↓
  DTOs      Use Cases    Entities, VOs, Repos (interfaces)
                              ↑
                        Infrastructure (implements interfaces)
```

**Rule**: Dependencies flow INWARD. Application imports Domain. Infrastructure imports Domain + Application (to implement interfaces). Domain imports nothing except common.

## Three-Plane Architecture

### Control Plane
- **OrchestrateMissionUseCase**: Mission lifecycle (Create → Planning → Ready → Running → Completed/Failed)
- **PolicyService**: Policy evaluation engine (allow/deny decisions)
- **TenantService**: Multi-tenant isolation and management
- **RBAC**: Role-based access control
- **Dashboard**: Aggregated metrics and status

### AI Plane
- **GeneratePlanUseCase**: LLM-driven attack plan generation with VectorStore context
- **AIMemoryService**: Semantic memory with embeddings and similarity search
- **GraphService**: Knowledge graph of entities and relationships
- **RecommendationService**: Scored and ranked security recommendations
- **ExplainabilityService**: Reasoning chains and evidence links

### Execution Plane
- **Recon**: Passive/active reconnaissance, OSINT gathering
- **Exploit**: Exploit matching, execution, session management
- **PostExploit**: Privilege escalation, lateral movement, data exfiltration
- **Report**: Finding aggregation, report generation (JSON/Markdown)

## Infrastructure Services

| Service | Description |
|---------|-------------|
| **EventStore** | Append-only event log with optimistic concurrency |
| **JobQueue** | Priority queue with dead-letter support |
| **Scheduler** | Cron-based and one-time job scheduling |
| **WorkflowEngine** | DAG execution with parallel nodes, retries, timeouts |
| **WorkerPool** | Distributed worker dispatch with heartbeat |
| **PluginManager** | Plugin lifecycle and dependency injection |
| **ConfigService** | YAML + env var configuration with hot reload |
| **SecretManager** | AES-256-GCM encrypted secret storage |
| **CacheStore** | Redis cache with tag-based invalidation |
| **HealthChecker** | Liveness/readiness probes |
| **LeaderElection** | In-process leader election abstraction |
| **CircuitBreaker** | State machine for external service resilience |
