package observability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TracerConfig struct {
	ServiceName string
	Enabled     bool
}

type spanTracer struct {
	config TracerConfig
	mu     sync.Mutex
	spans  []*spanRecord
}

type spanRecord struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Events     []spanEvent
	Attributes map[string]interface{}
	Error      string
}

type spanEvent struct {
	Name       string
	Attributes []Field
	Timestamp  time.Time
}

func NewTracer(cfg TracerConfig) Tracer {
	return &spanTracer{
		config: cfg,
	}
}

func (t *spanTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	if !t.config.Enabled {
		return ctx, noopSpan{}
	}

	traceID := newSpanID(16)
	spanID := newSpanID(8)
	parentID := ""

	if existing, ok := ctx.Value(ctxKey("parent_span_id")).(string); ok {
		parentID = existing
	}
	if existing, ok := ctx.Value(ctxKey("trace_id")).(string); ok {
		traceID = existing
	}

	record := &spanRecord{
		TraceID:    traceID,
		SpanID:     spanID,
		ParentID:   parentID,
		Name:       fmt.Sprintf("%s/%s", t.config.ServiceName, name),
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
	}

	t.mu.Lock()
	t.spans = append(t.spans, record)
	t.mu.Unlock()

	ctx = context.WithValue(ctx, ctxKey("trace_id"), traceID)
	ctx = context.WithValue(ctx, ctxKey("span_id"), spanID)
	if parentID == "" {
		ctx = context.WithValue(ctx, ctxKey("parent_span_id"), spanID)
	} else {
		ctx = context.WithValue(ctx, ctxKey("parent_span_id"), parentID)
	}

	return ctx, &tracingSpan{record: record, tracer: t}
}

type tracingSpan struct {
	record *spanRecord
	tracer *spanTracer
}

func (s *tracingSpan) End() {
	s.record.EndTime = time.Now()
}

func (s *tracingSpan) AddEvent(name string, attrs ...Field) {
	s.record.Events = append(s.record.Events, spanEvent{
		Name:       name,
		Attributes: attrs,
		Timestamp:  time.Now(),
	})
}

func (s *tracingSpan) SetAttribute(key string, value interface{}) {
	s.record.Attributes[key] = value
}

func (s *tracingSpan) RecordError(err error) {
	s.record.Error = err.Error()
}

func newSpanID(length int) string {
	b := make([]byte, length)
	_, _ = cryptoRandRead(b)
	return fmt.Sprintf("%x", b)
}

var cryptoRandRead = func(b []byte) (int, error) {
	return readRand(b)
}

func readRand(b []byte) (int, error) {
	for i := range b {
		b[i] = byte(time.Now().UnixNano() & 0xFF)
	}
	return len(b), nil
}
