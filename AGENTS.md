# AGENTS.md — Ophidian3 Development Guide

## Project Overview

Ophidian3 is an internal, solo-developer AI Offensive Security Platform.

Primary goals:

- Production-first engineering
- Clean Architecture
- Domain Driven Design
- CQRS
- Event Sourcing
- Hexagonal Architecture
- Three Plane Architecture

This project prioritizes architecture, correctness, maintainability, and stealth over rapid feature development. This is a personal operational tool, not a commercial SaaS.

---

# Working Agreement

Before making any code changes:

- Understand the existing implementation.
- Prefer extending existing components over rewriting.
- Preserve backward compatibility unless explicitly instructed.
- Do not refactor unrelated code.
- Ask for clarification if architecture intent is unclear.

---

# When uncertain:

- Read the surrounding code first.
- Search for existing implementations.
- Reuse established patterns.
- Avoid introducing new abstractions unless justified.

---

# When responding:

- Be concise.
- Reference package/file/function.
- Explain reasoning.
- Avoid unnecessary rewrites.
- Preserve project style.

---

# Technology Stack

- Go 1.22+
- PostgreSQL (pgxpool)
- Echo v4
- Redis (optional cache/queue)
- NATS (internal event bus)
- Ollama / DeepSeek (Local/Cloud LLM)
- YAML configuration
- Bubble Tea (TUI)
- Cobra (CLI)

---

# Architecture Principles (Non-Negotiable)

Follow these principles at all times.

- Clean Architecture
- Domain Driven Design
- CQRS
- Event Sourcing
- Ports and Adapters
- Dependency Inversion
- Composition over inheritance

Never violate these principles.

---

# Three Plane Architecture

## Control Plane

Responsible for:

- REST API
- TUI Dashboard
- Mission orchestration
- Policy and RoE (Rules of Engagement) validation
- Event dispatch
- Workflow coordination

Must NEVER:

- Execute exploits
- Run scanners
- Make autonomous AI decisions

## Execution Plane

Responsible for:

- Recon (Nmap, Chromedp)
- Exploitation
- Tool execution
- Post exploitation
- Reporting

Must NEVER:

- Decide strategy
- Change mission objectives
- Bypass RoE without explicit Control Plane approval

## AI Plane

Responsible for:

- Recommendations
- Planning
- Ranking
- Summarization
- Context reasoning
- Memory retrieval (via chromem-go)

Must NEVER:

- Execute tools
- Execute commands
- Modify infrastructure
- Bypass policy

AI is always an Advisor.

Never a Commander.

---

# Event Sourcing

Every state transition MUST produce a Domain Event.

The Event Store (PostgreSQL) is the single source of truth.

Aggregate state is reconstructed by replaying events.

Never mutate aggregate state outside aggregate methods.

Never bypass Event Store.

---

# Dependency Rules

Dependency direction:

Domain

down to

Application

down to

Infrastructure

down to

Interfaces (cmd/web)

Never reverse dependencies.

Domain must never import Application or Infrastructure.

Application must never depend on Infrastructure implementations directly (use interfaces).

Infrastructure implements Application interfaces.

---

# Repository Rules

Repositories interfaces belong to Domain.

Implementations belong to Infrastructure (e.g., postgres package).

Never expose database types (like pgx.Rows) outside Infrastructure.

---

# Coding Guidelines

Prefer:

- Composition
- Small interfaces
- Explicit constructors
- Context propagation
- Wrapped errors
- Immutable value objects

Avoid:

- Global state
- Panic
- Reflection unless strictly necessary
- Hidden side effects
- God objects
- Premature optimization

---

# Error Handling

Wrap infrastructure errors using fmt.Errorf("context: %w", err).

Return domain errors unchanged.

Always preserve error chains.

Use context cancellation correctly for long-running scans or LLM calls.

---

# Performance

Prefer:

- Context cancellation
- Bounded goroutines (worker pools)
- Connection pooling (pgxpool)

Avoid:

- Goroutine leaks
- Unbounded channels
- Large allocations in hot paths
- Mutex contention
- Blocking operations in the Event Loop

---

# Security

Never:

- Hardcode secrets (use .env or vault)
- Disable TLS validation in production
- Log credentials or raw exploit payloads to standard logs
- Ignore context cancellation
- Ignore Rules of Engagement

---

# Build and Verify

Run before committing:

go build ./cmd/...

go vet ./...

go test ./internal/domain/...

Run integration tests when Infrastructure changes.

---

# Startup Order

1. PostgreSQL
2. Redis/NATS (if used)
3. ophidian-worker
4. ophidian-server
5. ophidian-cli dashboard

Verify health endpoints before testing.

---

# Project Structure

cmd/
internal/
  domain/
  application/
  infrastructure/
  interfaces/
pkg/
configs/
docs/

Refer to ARCHITECTURE.md for complete directory documentation.

---

# Development Workflow

Before coding:

1. Read AGENTS.md
2. Read DEVELOPMENT_STATUS.md
3. Read ARCHITECTURE.md
4. Understand current phase
5. Verify architecture boundaries

After coding:

1. Build
2. Vet
3. Test
4. Review diff
5. Commit

---

# Code Review Rules

Review priority:

1. Architecture
2. Three Plane separation
3. DDD boundaries
4. Event Sourcing integrity
5. Dependency direction
6. CQRS consistency
7. Concurrency safety
8. Security
9. Performance
10. Maintainability

Never prioritize formatting over architecture.

Always reference the specific package, file, and function.

Explain WHY something should change, not just WHAT.

---

# Forbidden Patterns

Never:

- Import Infrastructure into Domain
- Bypass Repository interfaces
- Bypass Event Store
- Place business logic in HTTP handlers
- Execute tools directly from AI Plane
- Introduce circular dependencies
- Break aggregate invariants
- Leave TODO comments or empty stubs without explicit logging

---

# Context Files

Read these when additional context is needed:

- ARCHITECTURE.md
- DEVELOPMENT_STATUS.md

ARCHITECTURE.md contains detailed system architecture and component mapping.

DEVELOPMENT_STATUS.md contains current implementation status, completed phases, known issues, and next milestones.
