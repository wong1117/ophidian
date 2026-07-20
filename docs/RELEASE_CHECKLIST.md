# Ophidian v1.0 Release Checklist

**Target Release Date:** TBD
**Current Status:** Release Candidate

---

## Pre-Release Verification

### Code Quality
- [ ] All 237 Go files compile without errors
- [ ] `make lint` passes (golangci-lint, 30 rules)
- [ ] `make test` passes (all unit tests)
- [ ] `make test-race` passes (no data races detected)
- [ ] `make test-coverage` shows >= 80% on application packages
- [ ] `make fuzz` completes without panics (7 fuzz targets)

### Architecture Compliance
- [ ] Dependency direction verified (application → domain, infrastructure → domain/app)
- [ ] No infrastructure imports in domain layer
- [ ] No circular dependencies
- [ ] Architecture lint (`make arch-lint`) passes

### Public API Review (537 exported types, 689 exported functions)
- [ ] All exported types documented with GoDoc comments
- [ ] No breaking API changes since last stable release
- [ ] Deprecated APIs annotated with `// Deprecated:` comments
- [ ] Error sentinel values stable (no new `common.Err*` without documentation)

### Backward Compatibility
- [ ] Plugin SDK interface backward compatible
- [ ] EventStore schema backward compatible (new columns nullable)
- [ ] REST API routes backward compatible (versioned under `/api/v1`)
- [ ] DTOs backward compatible (new fields added with `omitempty`)
- [ ] Domain event structures backward compatible
- [ ] Repository interfaces backward compatible

### Database & Migrations
- [ ] EventStore migration scripts create all required tables
- [ ] RBAC migration scripts (users, roles, permissions)
- [ ] Feature flags migration scripts
- [ ] Knowledge graph migration scripts
- [ ] Tenant migration scripts
- [ ] All migrations idempotent (use `IF NOT EXISTS`)

### Deployment
- [ ] Dockerfile builds successfully
- [ ] Helm chart (`deploy/helm/ophidian/`) validates
- [ ] All 8 Helm templates render correctly
- [ ] ConfigMap and Secret properly separated
- [ ] Liveness and readiness probes configured
- [ ] Resource limits and HPA configured
- [ ] PDB configured for high availability

### Performance
- [ ] EventStore Append: 513K ops/s ✓
- [ ] Queue Enqueue: 212K ops/s ✓
- [ ] Workflow 3-node linear: 42K ops/s ✓
- [ ] Cache Set: 536K ops/s ✓
- [ ] All benchmarks within 10% of baseline

### Documentation
- [ ] Architecture overview (`docs/architecture/OVERVIEW.md`)
- [ ] Developer guide (`docs/DEVELOPER_GUIDE.md`)
- [ ] Release engineering (`docs/RELEASE_ENGINEERING.md`)
- [ ] Performance report (`docs/PERFORMANCE_REPORT.md`)
- [ ] Quality report (`docs/QUALITY_REPORT.md`)
- [ ] Helm chart README (`deploy/helm/ophidian/README.md`)
- [ ] Examples (`examples/main.go`) runnable

### Observability
- [ ] Health checks implement `ha.Checker` interface
- [ ] Structured logging via `observability.Logger`
- [ ] Metrics via `observability.Metrics`
- [ ] Tracing via `observability.Tracer`
- [ ] Request ID and Correlation ID propagation
- [ ] Error middleware maps domain errors to HTTP status codes

### Operations
- [ ] Rollback procedure documented
- [ ] Database backup procedure tested
- [ ] Secret rotation procedure verified
- [ ] Graceful shutdown (SIGTERM) works correctly
- [ ] Graceful startup (readiness probe) works correctly

---

## Remaining Blockers for v1.0

### Critical (Must Fix)
| # | Issue | Location | Impact |
|---|-------|----------|--------|
| B1 | Feature flag cache data race | `feature_service.go:16` | Crash under concurrent access |
| B2 | Audit service imports infrastructure | `audit_service.go:12` | Architecture violation |
| B3 | Queue uses `ctx interface{}` | `queue.go:63` | No cancellation propagation |
| B4 | GhostSession Events slice race | `ghost/collaboration.go` | Data corruption |
| B5 | Helm secret rotation on every upgrade | `secret.yaml:9-12` | Production outage on upgrade |

### High (Should Fix)
| # | Issue | Location | Impact |
|---|-------|----------|--------|
| H1 | Dashboard cache key missing tenant | `dashboard_service.go:47` | Cross-tenant data leak |
| H2 | Workflow goroutine leak | `engine.go:204` | Memory leak |
| H3 | Scheduler mutex + DB I/O | `scheduler.go:268` | Performance + availability |
| H4 | Tracer unbounded memory | `tracer.go:71` | OOM under load |
| H5 | 13 packages at 0% test coverage | various | Reliability risk |

### Medium (Should Fix)
| # | Issue | Location | Impact |
|---|-------|----------|--------|
| M1 | 35+ bare error returns | 15 files | Debugging difficulty |
| M2 | DomainEvent duplicated 5 times | Domain packages | Maintenance burden |
| M3 | Dashboard 3 stat groups are zero | `dashboard_service.go` | Incomplete feature |
| M4 | Backup incremental mode broken | `backup_service.go:127` | Feature not working |
| M5 | Plaintext secrets in Config struct | `config.go:38` | Security risk |

---

## Release Artifacts

| Artifact | Platform | Format |
|----------|----------|--------|
| ophidian-server | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 | Binary (Go static) |
| ophidian-cli | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 | Binary (Go static) |
| ophidian-agent | linux/amd64 | Binary |
| ophidian-worker | linux/amd64 | Binary |
| Docker image | linux/amd64 | ghcr.io/ophidian/ophidian:v1.0.0 |
| Helm chart | Kubernetes 1.24+ | tar.gz |
| SBOM | All platforms | SPDX JSON |

---

## Sign-off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Lead Engineer | | | |
| Security Review | | | |
| QA Lead | | | |
| Release Manager | | | |

---

**Decision:** ☐ Ready for Release   ☐ Not Ready (Blocker List Attached)
