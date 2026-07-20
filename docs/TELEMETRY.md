# Ophidian Telemetry Configuration

## Prometheus Configuration

Add the following to your Prometheus `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'ophidian'
    scrape_interval: 15s
    static_configs:
      - targets: ['ophidian-server:8080']
        labels:
          service: 'ophidian'
          environment: 'production'
    metrics_path: '/metrics'
```

## Grafana Configuration

1. Add Prometheus as a data source in Grafana (URL: `http://prometheus:9090`)
2. Import dashboards from `deploy/grafana/dashboards/`:
   - `service-overview.json` — Request rate, latency percentiles, error rate, DB load, queue depth, workers
3. Set dashboard refresh interval to 10s

## Helm Integration

Add to `values.yaml`:

```yaml
telemetry:
  enabled: true
  otlpEndpoint: "http://otel-collector:4317"
  prometheusPort: 9090
  sampleRate: 1.0
```

## Dashboard Panels

### Service Overview (6 panels)
| Panel | Metric | Type |
|-------|--------|------|
| Request Rate | `rate(ophidian_http_requests_total[5m])` | Timeseries |
| Latency p50/p95/p99 | `histogram_quantile(..., ophidian_http_request_duration_seconds_bucket)` | Timeseries |
| Error Rate | `rate(ophidian_http_errors_total[5m])` | Timeseries |
| DB Load | `rate(ophidian_db_query_duration_seconds_count[5m])` | Timeseries |
| Queue Depth | `ophidian_queue_depth` | Timeseries |
| Active Workers | `ophidian_*_active_workers` | Timeseries |

## Key Metrics Exposed

| Metric | Type | Description |
|--------|------|-------------|
| `ophidian_http_requests_total` | Counter | Total HTTP requests |
| `ophidian_http_errors_total` | Counter | Total HTTP errors |
| `ophidian_http_request_duration_seconds` | Histogram | HTTP request latency |
| `ophidian_db_query_duration_seconds` | Histogram | DB operation latency |
| `ophidian_db_write_duration_seconds` | Histogram | DB write latency |
| `ophidian_queue_depth` | Gauge | Queue depth |
| `ophidian_*_active_workers` | Gauge | Active worker count per handler |

## Service Dependency Graph

```
                          ┌──────────────┐
                          │   Gateway    │
                          │  (Ingress)   │
                          └──────┬───────┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
               ┌────▼───┐   ┌───▼────┐  ┌───▼────┐
               │  API   │   │ Worker │  │ Agent  │
               │ Server │   │ Pool   │  │        │
               └───┬────┘   └───┬────┘  └───┬────┘
                   │            │            │
          ┌────────┼────────────┼────────────┼────────┐
          │        │            │            │        │
     ┌────▼──┐ ┌──▼───┐  ┌────▼────┐  ┌───▼───┐ ┌──▼───┐
     │Postgre│ │Redis │  │  NATS   │  │  AI   │ │Sched │
     │  SQL  │ │      │  │         │  │Provid.│ │ uler │
     └───────┘ └──────┘  └─────────┘  └───────┘ └──────┘
```

## Tracing with OpenTelemetry

Spans are propagated through:
- HTTP headers: `traceparent`, `tracestate`
- Context: `correlation_id`, `request_id`
- Service name: `ophidian/{service-name}`

Example span hierarchy:
```
ophidian/api-server/PlanGeneration
  ├── ophidian/api-server/VectorStoreSearch
  ├── ophidian/api-server/LLMGenerate
  │   └── ophidian/ai-provider/openai/Completions
  └── ophidian/api-server/ValidatePlan
```

## Alerts

Recommended Prometheus alerting rules:

```yaml
groups:
  - name: ophidian
    rules:
      - alert: HighErrorRate
        expr: rate(ophidian_http_errors_total[5m]) > 1
        for: 5m
        labels: { severity: critical }
        annotations: { summary: "Error rate > 1/s for 5 minutes" }

      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(ophidian_http_request_duration_seconds_bucket[5m])) > 5
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "p99 latency > 5s for 5 minutes" }

      - alert: DeepQueue
        expr: ophidian_queue_depth > 1000
        for: 10m
        labels: { severity: warning }
        annotations: { summary: "Queue depth > 1000 for 10 minutes" }
```
