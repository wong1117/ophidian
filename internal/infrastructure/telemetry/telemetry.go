package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/infrastructure/observability"
)

type TelemetryConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	PrometheusPort int
	Enabled        bool
	SampleRate     float64
}

type Telemetry struct {
	config   TelemetryConfig
	tracer   observability.Tracer
	metrics  *MetricsRegistry
	mu       sync.RWMutex
}

type MetricsRegistry struct {
	counters   map[string]*Counter
	histograms map[string]*Histogram
	gauges     map[string]*Gauge
	mu         sync.RWMutex
}

type Counter struct {
	Name   string
	Help   string
	Labels map[string]string
	Value  int64
	mu     sync.Mutex
}

type Histogram struct {
	Name    string
	Help    string
	Buckets []float64
	Labels  map[string]string
	Sum     float64
	Count   int64
	Min     float64
	Max     float64
	mu      sync.Mutex
}

type Gauge struct {
	Name   string
	Help   string
	Labels map[string]string
	Value  float64
	mu     sync.Mutex
}

func NewTelemetry(cfg TelemetryConfig) *Telemetry {
	if !cfg.Enabled {
		return &Telemetry{config: cfg}
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 1.0
	}

	return &Telemetry{
		config: cfg,
		tracer: observability.NewTracer(observability.TracerConfig{
			ServiceName: cfg.ServiceName,
			Enabled:     true,
		}),
		metrics: &MetricsRegistry{
			counters:   make(map[string]*Counter),
			histograms: make(map[string]*Histogram),
			gauges:     make(map[string]*Gauge),
		},
	}
}

func (t *Telemetry) StartSpan(ctx context.Context, name string) (context.Context, observability.Span) {
	if t.tracer == nil {
		return ctx, observability.NoopSpan{}
	}
	ctx = observability.WithCorrelationID(ctx, observability.NewCorrelationID())
	ctx = observability.WithRequestID(ctx, observability.NewRequestID())
	return t.tracer.StartSpan(ctx, fmt.Sprintf("%s/%s", t.config.ServiceName, name))
}

func (t *Telemetry) Counter(name, help string) *Counter {
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()

	if c, ok := t.metrics.counters[name]; ok {
		return c
	}
	c := &Counter{
		Name:   fmt.Sprintf("%s_%s", t.config.ServiceName, name),
		Help:   help,
		Labels: t.defaultLabels(),
	}
	t.metrics.counters[name] = c
	return c
}

func (t *Telemetry) Histogram(name, help string, buckets []float64) *Histogram {
	if len(buckets) == 0 {
		buckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	}
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()

	if h, ok := t.metrics.histograms[name]; ok {
		return h
	}
	h := &Histogram{
		Name:    fmt.Sprintf("%s_%s", t.config.ServiceName, name),
		Help:    help,
		Buckets: buckets,
		Labels:  t.defaultLabels(),
	}
	t.metrics.histograms[name] = h
	return h
}

func (t *Telemetry) Gauge(name, help string) *Gauge {
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()

	if g, ok := t.metrics.gauges[name]; ok {
		return g
	}
	g := &Gauge{
		Name:   fmt.Sprintf("%s_%s", t.config.ServiceName, name),
		Help:   help,
		Labels: t.defaultLabels(),
	}
	t.metrics.gauges[name] = g
	return g
}

func (t *Telemetry) defaultLabels() map[string]string {
	return map[string]string{
		"service":     t.config.ServiceName,
		"version":     t.config.ServiceVersion,
		"environment": t.config.Environment,
	}
}

func (c *Counter) Inc() {
	c.mu.Lock()
	c.Value++
	c.mu.Unlock()
}

func (c *Counter) Add(n int64) {
	c.mu.Lock()
	c.Value += n
	c.mu.Unlock()
}

func (c *Counter) Snapshot() (string, map[string]string, int64) {
	c.mu.Lock()
	v := c.Value
	c.mu.Unlock()
	return c.Name, c.Labels, v
}

func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	h.Count++
	h.Sum += v
	if v < h.Min || h.Count == 1 {
		h.Min = v
	}
	if v > h.Max {
		h.Max = v
	}
	h.mu.Unlock()
}

func (h *Histogram) Snapshot() (string, map[string]string, HistogramSnapshot) {
	h.mu.Lock()
	s := HistogramSnapshot{
		Count: h.Count,
		Sum:   h.Sum,
		Min:   h.Min,
		Max:   h.Max,
	}
	h.mu.Unlock()
	return h.Name, h.Labels, s
}

type HistogramSnapshot struct {
	Count int64
	Sum   float64
	Min   float64
	Max   float64
}

func (g *Gauge) Set(v float64) {
	g.mu.Lock()
	g.Value = v
	g.mu.Unlock()
}

func (g *Gauge) Snapshot() (string, map[string]string, float64) {
	g.mu.Lock()
	v := g.Value
	g.mu.Unlock()
	return g.Name, g.Labels, v
}

func (t *Telemetry) RecordRequestLatency(endpoint string, d time.Duration) {
	if t.metrics == nil {
		return
	}
	h := t.Histogram("http_request_duration_seconds", "HTTP request latency", nil)
	h.Observe(d.Seconds())

	c := t.Counter("http_requests_total", "Total HTTP requests")
	c.Inc()
}

func (t *Telemetry) RecordRequestError(endpoint string) {
	if t.metrics == nil {
		return
	}
	c := t.Counter("http_errors_total", "Total HTTP errors")
	c.Inc()
}

func (t *Telemetry) RecordDBLatency(operation string, d time.Duration) {
	if t.metrics == nil {
		return
	}
	h := t.Histogram(fmt.Sprintf("db_%s_duration_seconds", operation), "DB operation latency", nil)
	h.Observe(d.Seconds())
}

func (t *Telemetry) RecordQueueDepth(queue string, depth int) {
	if t.metrics == nil {
		return
	}
	g := t.Gauge(fmt.Sprintf("%s_depth", queue), "Queue depth")
	g.Set(float64(depth))
}

func (t *Telemetry) RecordActiveWorkers(handler string, count int) {
	if t.metrics == nil {
		return
	}
	g := t.Gauge(fmt.Sprintf("%s_active_workers", handler), "Active workers")
	g.Set(float64(count))
}

func (t *Telemetry) Handler() http.Handler {
	return t.metrics.Handler()
}

func (m *MetricsRegistry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		m.mu.RLock()
		defer m.mu.RUnlock()

		for _, c := range m.counters {
			name, labels, val := c.Snapshot()
			fmt.Fprintf(w, "# HELP %s %s\n", name, c.Help)
			fmt.Fprintf(w, "# TYPE %s counter\n", name)
			fmt.Fprintf(w, "%s{%s} %d\n", name, formatLabels(labels), val)
		}

		for _, h := range m.histograms {
			name, labels, snap := h.Snapshot()
			fmt.Fprintf(w, "# HELP %s %s\n", name, h.Help)
			fmt.Fprintf(w, "# TYPE %s histogram\n", name)
			fmt.Fprintf(w, "%s_count{%s} %d\n", name, formatLabels(labels), snap.Count)
			fmt.Fprintf(w, "%s_sum{%s} %f\n", name, formatLabels(labels), snap.Sum)
		}

		for _, g := range m.gauges {
			name, labels, val := g.Snapshot()
			fmt.Fprintf(w, "# HELP %s %s\n", name, g.Help)
			fmt.Fprintf(w, "# TYPE %s gauge\n", name)
			fmt.Fprintf(w, "%s{%s} %f\n", name, formatLabels(labels), val)
		}
	})
}

func formatLabels(labels map[string]string) string {
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
	}
	return joinStrings(parts, ",")
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}
