# OPHIDIAN Architecture

## Three-Plane Architecture

- **Control Plane**: Mission lifecycle, task scheduling, RoE enforcement
- **AI Plane**: Reasoning, planning, strategy adaptation
- **Execution Plane**: Recon, exploit, post-exploit, reporting

## Clean Architecture Layers

1. **Domain Core** - Pure Go, zero external dependencies
2. **Domain** - Business logic, entities, repository interfaces
3. **Application** - Use cases, service implementations
4. **Infrastructure** - Adapters, external dependencies

## Key Patterns

- Event Sourcing for immutable audit trail
- Saga Pattern for distributed transactions
- Circuit Breaker for external service resilience
- CQRS for AI reasoning vs execution separation
