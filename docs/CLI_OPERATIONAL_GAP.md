# Ophidian CLI — Operational Gap Report

## Current State

The CLI (`ophidian-cli`) is an **infrastructure management tool**. It does NOT have operational commands for running missions, creating tasks, or triggering offensive workflows.

### Available Commands (infrastructure only)

| Command | Purpose |
|---------|---------|
| `dashboard` | Interactive TUI (read-only metrics/logs viewer) |
| `workflow [name]` | Animated workflow monitor |
| `events` | Live event stream viewer |
| `metrics` | System metrics table |
| `plugins` | Plugin manager listing |
| `scaffold` | Project scaffolding generator |
| `deploy` | Deployment progress bar |
| `migrate` | Database migration runner |
| `version` | Version info |

### Missing Operational Commands

The following commands do NOT exist and are not planned for v1.0:

| Command | Gap |
|---------|-----|
| `mission create` | Not implemented — use `curl` to the REST API |
| `mission list` | Not implemented — use `curl` to the REST API |
| `recon start` | Not implemented — use `curl` to the REST API |
| `exploit execute` | Not implemented — use `curl` to the REST API |
| `task submit` | Not implemented — no task creation workflow |
| `agent deploy` | Not implemented — no agent management |

### Workaround

The REST API at `http://localhost:8443/api/v1/` provides full mission management. Use `curl` commands documented in `docs/API_INTEGRATION_GUIDE.md` to trigger workflows programmatically.

### Why?

The CLI was designed for infrastructure monitoring (dashboard, metrics, events) and development tooling (scaffold, deploy, migrate). Operational commands were intentionally deferred to v2.0 to keep the CLI scope manageable. The REST API provides equivalent functionality.
