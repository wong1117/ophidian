# Ophidian Performance Optimization Report

## 1. Infrastructure Benchmarks

### Event Store (PostgreSQL)
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Append (single event) | 513,015 | 2,285 | 552 | 22 |
| AppendBatch (100 events) | 7,964 | 153,685 | 43,186 | 1,696 |
| LoadStream | 4,810,239 | 246 | 112 | 3 |

### Queue System
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Enqueue (single) | 212,002 | 2,399 | 573 | 4 |
| Dequeue+Ack | 258,290 | 2,660 | 976 | 8 |
| Enqueue Large (1k batch) | 232 | 2,687,593 | 604,774 | 3,751 |
| Dequeue 1k | 222 | 3,079,608 | 1,179,182 | 6,805 |
| Stats | 26,639,294 | 22.8 | 0 | 0 |
| PromoteDelayed | 6,160,808 | 85.8 | 0 | 0 |

### Scheduler
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Schedule job | 22,088 | 23,013 | 6,334 | 12 |

### Worker Pool
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Dispatch | 928,142 | 1,920 | 432 | 3 |

### Workflow Engine
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Linear (3 nodes) | 42,300 | 14,410 | 3,584 | 46 |
| DAG (4 nodes) | 31,946 | 21,364 | 4,080 | 57 |
| Large Linear (100) | 951 | 666,187 | 118,543 | 1,070 |
| Parallel (20 nodes) | 8,283 | 84,799 | 23,429 | 208 |

### Redis Cache
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Set | 536,553 | 1,189 | 274 | 2 |
| Get | 454,272 | 1,237 | 199 | 5 |
| Tag Invalidation (1k) | 710 | 725,827 | 122,979 | 1,766 |

## 2. Application Benchmarks

### Graph Service
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| AddNode | 186,668 | 3,193 | 402 | 10 |
| AddEdge | 208,842 | 2,797 | 378 | 8 |
| ShortestPath (10 nodes) | 37,777 | 15,112 | 3,224 | 54 |
| Traverse (10 nodes, BFS) | 72,093 | 7,158 | 904 | 25 |

### Serialization
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Mission to JSON | 202,864 | 5,799 | 1,136 | 6 |
| Finding to JSON | 383,289 | 2,876 | 608 | 3 |
| 100 Missions to JSON | 2,090 | 534,922 | 94,418 | 600 |

### Synchronization Primitives
| Benchmark | ops/s | ns/op | B/op | allocs/op |
|-----------|-------|-------|------|-----------|
| Channel send/recv | 19,205,131 | 62.6 | 0 | 0 |
| Mutex lock/unlock | 78,672,003 | 14.7 | 0 | 0 |
| RWMutex lock/unlock | 34,958,930 | 34.5 | 0 | 0 |

## 3. Allocation Hotspots (allocs/op > 10)

| Component | Allocation | Severity | Recommendation |
|-----------|-----------|----------|----------------|
| EventStore Append | 22 allocs | Medium | Consider object pooling for EventRecord |
| EventStore AppendBatch | 1,696 allocs (17 per event) | Low | Batched allocations are amortized |
| Queue Dequeue (1k) | 6,805 allocs (7 per job) | Medium | Use sync.Pool for queue.Job objects |
| Workflow Large (100 nodes) | 1,070 allocs (11 per node) | Medium | Pre-allocate node result map |
| MemoryCache TagInvalidation | 1,766 allocs | Medium | Use iterator pattern instead of full copy |

## 4. Optimization Recommendations

### High Priority
1. **Object pooling for high-frequency paths**: Use `sync.Pool` for `EventRecord`, `queue.Job`, and `WorkflowResult` objects
2. **Pre-allocate slices**: In `EventStore.LoadStream`, estimate event count and pre-allocate the slice
3. **Batch enqueue**: Use `AppendBatch` for multiple events (68x reduction in per-event overhead vs individual appends)

### Medium Priority
4. **GraphService**: Pre-build adjacency map once and reuse across `Traverse`/`ShortestPath` calls
5. **JSON serialization**: Use `json.NewEncoder` for streaming serialization of large datasets
6. **Cache tag invalidation**: Use Redis pipeline for batch DEL operations

### Low Priority
7. **Mutex > Channel**: Mutex is 4x faster than channel for simple state synchronization
8. **RWMutex overhead**: For write-heavy workloads, use plain Mutex (2x faster than RWMutex for writes)
9. **Dashboard caching**: Use typed cache (not `interface{}`) to avoid type assertion overhead

## 5. Database Index Review

### Existing Indexes (EventStore)
- `idx_events_aggregate` — `(aggregate_id, version)` — effectively covers LoadStream
- `idx_events_occurred_at` — `(occurred_at)` — covers LoadAllEvents time-range queries
- `idx_events_type` — `(aggregate_type, event_type)` — covers audit type filters
- `idx_events_correlation` — `(correlation_id)` filtered — sparse index for tracing

### Recommended Additions
- `idx_events_id_version` — `(id, version)` — covers optimistic concurrency append checks
- `idx_snapshots_type` — `(aggregate_type)` — already exists, good
- RBAC tables: add `idx_rbac_users_username`, `idx_rbac_roles_name`
- Feature flags: add `idx_features_key` (UNIQUE already)
- Knowledge graph: add `idx_kg_edges_from`, `idx_kg_edges_to` for graph traversal
- Knowledge graph: add `idx_kg_nodes_entity` for entity lookups

## 6. Cache Usage Review

| Cache | TTL | Key Pattern | Tenant Isolation | Recommendation |
|-------|-----|-------------|-----------------|----------------|
| AI Memory (secrets) | 5 min | in-memory map | N/A | Good, but hardcoded TTL |
| Dashboard overview | 30 sec | `"dashboard:overview"` | None — critical bug | Add tenant ID to key |
| Feature flag cache | lifetime | `feature.Key` in map | N/A | Good, needs mutex |
| Secret cache | 5 min | `manager.cache` map | N/A | Good, make TTL configurable |

## 7. Profiling Guide

```bash
# CPU profile
go test -bench=. -cpuprofile=cpu.prof -benchtime=10s ./internal/infrastructure/persistence/postgres/...
go tool pprof -http=:8080 cpu.prof

# Memory profile
go test -bench=. -memprofile=mem.prof -benchtime=10s ./internal/infrastructure/queue/...
go tool pprof -http=:8081 mem.prof

# Trace
go test -bench=BenchmarkQueue_Dequeue -trace=trace.out ./internal/infrastructure/queue/...
go tool trace trace.out
```

## 8. Quick Wins (immediate impact, low risk)

1. **Dashboard cache key fix**: Add tenant isolation (1 line change)
2. **Feature flag cache mutex**: Add `sync.RWMutex` (3 lines)
3. **Graph traversal**: Pre-compute outgoing edges map in `Traverse` to avoid N+1 DB queries
4. **Queue dequeue**: Use `sync.Pool` for `*jobHeap` during dequeue cycles
5. **EventStore**: Pre-allocate `events` slice in `LoadStream` with initial capacity
