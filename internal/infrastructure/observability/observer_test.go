package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStructuredLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerConfig{
		Level:  LevelDebug,
		Output: &buf,
	})

	ctx := context.Background()

	logger.Debug(ctx, "debug msg", StrField("key", "val"))
	logger.Info(ctx, "info msg")
	logger.Warn(ctx, "warn msg")
	logger.Error(ctx, "error msg", ErrField(errors.New("test error")))

	output := buf.String()
	assert.Contains(t, output, "debug msg")
	assert.Contains(t, output, "info msg")
	assert.Contains(t, output, "warn msg")
	assert.Contains(t, output, "error msg")
	assert.Contains(t, output, "test error")
}

func TestStructuredLogger_LevelFilter(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerConfig{
		Level:  LevelWarn,
		Output: &buf,
	})

	ctx := context.Background()

	logger.Debug(ctx, "debug msg")
	logger.Info(ctx, "info msg")
	logger.Warn(ctx, "warn msg")

	output := buf.String()
	assert.NotContains(t, output, "debug msg")
	assert.NotContains(t, output, "info msg")
	assert.Contains(t, output, "warn msg")
}

func TestStructuredLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerConfig{
		Level:  LevelInfo,
		Output: &buf,
	})

	ctx := context.Background()
	logger.Info(ctx, "test", StrField("user", "admin"), IntField("count", 5))

	output := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(output, "{"), "expected JSON output")
	assert.Contains(t, output, `"level":"INFO"`)
	assert.Contains(t, output, `"message":"test"`)
	assert.Contains(t, output, `"user":"admin"`)
	assert.Contains(t, output, `"count":5`)
}

func TestStructuredLogger_With(t *testing.T) {
	var buf bytes.Buffer

	base := NewLogger(LoggerConfig{Level: LevelInfo, Output: &buf})
	enriched := base.With(StrField("service", "api"), StrField("env", "test"))

	ctx := context.Background()
	enriched.Info(ctx, "request")

	output := buf.String()
	assert.Contains(t, output, `"service":"api"`)
	assert.Contains(t, output, `"env":"test"`)
}

func TestStructuredLogger_WithContextFields(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(LoggerConfig{Level: LevelInfo, Output: &buf})

	ctx := WithCorrelationID(context.Background(), "corr-123")
	ctx = WithRequestID(ctx, "req-456")

	logger.Info(ctx, "contexted")

	output := buf.String()
	assert.Contains(t, output, `"correlation_id":"corr-123"`)
	assert.Contains(t, output, `"request_id":"req-456"`)
}

func TestContextHelpers_RequestID(t *testing.T) {
	id := NewRequestID()
	assert.Len(t, id, 16)

	ctx := WithRequestID(context.Background(), id)
	assert.Equal(t, id, RequestID(ctx))
}

func TestContextHelpers_CorrelationID(t *testing.T) {
	id := NewCorrelationID()
	assert.Len(t, id, 32)

	ctx := WithCorrelationID(context.Background(), id)
	assert.Equal(t, id, CorrelationID(ctx))
}

func TestExtractCorrelationFields(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "corr-1")
	ctx = WithRequestID(ctx, "req-1")

	fields := ExtractCorrelationFields(ctx)
	assert.Len(t, fields, 2)
	assert.Equal(t, "correlation_id", fields[0].Key)
	assert.Equal(t, "request_id", fields[1].Key)
}

func TestExtractCorrelationFields_Empty(t *testing.T) {
	ctx := context.Background()
	fields := ExtractCorrelationFields(ctx)
	assert.Empty(t, fields)
}

func TestHelperFields(t *testing.T) {
	assert.Equal(t, "key", IntField("key", 42).Key)
	assert.Equal(t, 42, IntField("key", 42).Value)
	assert.Equal(t, "key", StrField("key", "val").Key)
	assert.Equal(t, "val", StrField("key", "val").Value)
	assert.Equal(t, 3.14, FloatField("key", 3.14).Value)
	assert.True(t, BoolField("key", true).Value.(bool))
}

func TestTracer_StartSpan_Disabled(t *testing.T) {
	tracer := NewTracer(TracerConfig{Enabled: false})
	ctx, span := tracer.StartSpan(context.Background(), "test")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)

	span.End()
	span.AddEvent("evt")
	span.SetAttribute("key", "val")
	span.RecordError(errors.New("err"))
}

func TestTracer_StartSpan_Enabled(t *testing.T) {
	tracer := NewTracer(TracerConfig{Enabled: true, ServiceName: "test-svc"})
	ctx, span := tracer.StartSpan(context.Background(), "operation")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)

	traceID, ok := ctx.Value(ctxKey("trace_id")).(string)
	assert.True(t, ok)
	assert.NotEmpty(t, traceID)

	spanID, ok := ctx.Value(ctxKey("span_id")).(string)
	assert.True(t, ok)
	assert.NotEmpty(t, spanID)

	span.SetAttribute("db.type", "postgres")
	span.AddEvent("query_start", StrField("sql", "SELECT 1"))
	span.RecordError(errors.New("timeout"))
	span.End()
}

func TestTracer_ChildSpan(t *testing.T) {
	tracer := NewTracer(TracerConfig{Enabled: true, ServiceName: "svc"})

	parentCtx, _ := tracer.StartSpan(context.Background(), "parent")
	childCtx, _ := tracer.StartSpan(parentCtx, "child")

	parentSpanID, _ := ctxValue(parentCtx, "span_id")
	childParentID, _ := ctxValue(childCtx, "parent_span_id")

	assert.Equal(t, parentSpanID, childParentID)
}

func ctxValue(ctx context.Context, key string) (string, bool) {
	v, ok := ctx.Value(ctxKey(key)).(string)
	return v, ok
}

func TestMetrics_Counter(t *testing.T) {
	m := NewMetrics(MetricsConfig{Enabled: true})
	collector := m.(*metricsCollector)

	ctx := context.Background()

	m.IncrementCounter(ctx, "http.requests", 1, StrField("method", "GET"))
	m.IncrementCounter(ctx, "http.requests", 1, StrField("method", "GET"))
	m.IncrementCounter(ctx, "http.requests", 2, StrField("method", "POST"))

	snapshot := collector.Snapshot()
	assert.Len(t, snapshot, 2)

	getMetric := findMetric(snapshot, "http.requests", "method", "GET")
	assert.NotNil(t, getMetric)
	assert.Equal(t, MetricCounter, getMetric.Type)
	assert.Equal(t, int64(2), getMetric.Count)
	assert.Equal(t, float64(2), getMetric.Value)
}

func TestMetrics_Histogram(t *testing.T) {
	m := NewMetrics(MetricsConfig{Enabled: true})
	collector := m.(*metricsCollector)

	ctx := context.Background()

	m.RecordHistogram(ctx, "db.latency", 10.5, StrField("op", "query"))
	m.RecordHistogram(ctx, "db.latency", 5.0, StrField("op", "query"))
	m.RecordHistogram(ctx, "db.latency", 20.0, StrField("op", "query"))

	snapshot := collector.Snapshot()
	assert.Len(t, snapshot, 1)

	metric := snapshot[0]
	assert.Equal(t, MetricHistogram, metric.Type)
	assert.Equal(t, int64(3), metric.Count)
	assert.Equal(t, 5.0, metric.Min)
	assert.Equal(t, 20.0, metric.Max)
	assert.InDelta(t, 11.833, metric.Value, 0.01)
}

func TestMetrics_Gauge(t *testing.T) {
	m := NewMetrics(MetricsConfig{Enabled: true})
	collector := m.(*metricsCollector)

	ctx := context.Background()

	m.SetGauge(ctx, "active.sessions", 5)
	m.SetGauge(ctx, "active.sessions", 3)

	snapshot := collector.Snapshot()
	assert.Len(t, snapshot, 1)
	assert.Equal(t, float64(3), snapshot[0].Value)
}

func TestMetrics_Disabled(t *testing.T) {
	m := NewMetrics(MetricsConfig{Enabled: false})
	collector := m.(*metricsCollector)

	ctx := context.Background()

	m.IncrementCounter(ctx, "test", 1)
	m.RecordHistogram(ctx, "test", 1.0)
	m.SetGauge(ctx, "test", 1.0)

	assert.Empty(t, collector.Snapshot())
}

func TestObserver_Composite(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{Level: LevelInfo, Output: &buf})
	tracer := NewTracer(TracerConfig{Enabled: true, ServiceName: "test"})
	metrics := NewMetrics(MetricsConfig{Enabled: true})

	obs := NewObserver(logger, tracer, metrics)

	ctx := context.Background()
	ctx, span := obs.StartSpan(ctx, "operation")

	logger.Info(ctx, "operation started")

	metrics.(*metricsCollector).IncrementCounter(ctx, "operations", 1)

	span.End()

	output := buf.String()
	assert.NotEmpty(t, output)

	var logEntry map[string]interface{}
	json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	assert.Equal(t, "operation started", logEntry["message"])
}

func TestObserver_NilTracer(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{Level: LevelInfo, Output: &buf})
	metrics := NewMetrics(MetricsConfig{Enabled: true})

	obs := NewObserver(logger, nil, metrics)

	ctx, span := obs.StartSpan(context.Background(), "test")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)

	span.End()
}

func findMetric(snapshots []MetricSnapshot, name, tagKey, tagValue string) *MetricSnapshot {
	for i, s := range snapshots {
		if s.Name == name {
			for _, t := range s.Tags {
				if t.Key == tagKey && t.Value == tagValue {
					return &snapshots[i]
				}
			}
		}
	}
	return nil
}

func TestMetricKey(t *testing.T) {
	key := metricKey("test", []Field{
		{Key: "method", Value: "GET"},
		{Key: "path", Value: "/api"},
	})
	assert.Equal(t, "test;method=GET;path=/api", key)

	keyNoTags := metricKey("simple", nil)
	assert.Equal(t, "simple", keyNoTags)
}
