# OPHIDIAN — Offensive AI Security Platform

Three-Plane + Clean Architecture + Event Sourcing

## Architecture

- **Control Plane**: Mission lifecycle, task scheduling, RoE enforcement
- **AI Plane**: Reasoning, planning, strategy adaptation (ADVISOR, not CONTROLLER)
- **Execution Plane**: Recon, exploit, post-exploit, reporting

## Quick Start

```bash
make build
./build/ophidian-server
```

## Project Structure
```
ophidian/
├── cmd/              # Entry points
├── internal/
│   ├── domain/       # Business logic (pure)
│   ├── application/  # Use cases
│   └── infrastructure/ # Adapters
├── pkg/              # Shared libraries
├── web/              # HTMX UI
└── configs/          # Configuration
```

## Key Principles
- AI as ADVISOR, not CONTROLLER
- Clean Architecture dependency rule
- Event sourcing for audit trail
- Circuit breaker for resilience
