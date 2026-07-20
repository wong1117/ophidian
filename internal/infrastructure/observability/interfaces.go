package observability

import (
	"context"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Field struct {
	Key   string
	Value interface{}
}

type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
	With(fields ...Field) Logger
}

type Tracer interface {
	StartSpan(ctx context.Context, name string) (context.Context, Span)
}

type Span interface {
	End()
	AddEvent(name string, attrs ...Field)
	SetAttribute(key string, value interface{})
	RecordError(err error)
}

type MetricType int

const (
	MetricCounter   MetricType = iota
	MetricHistogram MetricType = iota
	MetricGauge     MetricType = iota
)

type Metrics interface {
	IncrementCounter(ctx context.Context, name string, value int64, tags ...Field)
	RecordHistogram(ctx context.Context, name string, value float64, tags ...Field)
	SetGauge(ctx context.Context, name string, value float64, tags ...Field)
}

type Observer struct {
	Logger  Logger
	Tracer  Tracer
	Metrics Metrics
}

func NewObserver(logger Logger, tracer Tracer, metrics Metrics) *Observer {
	return &Observer{
		Logger:  logger,
		Tracer:  tracer,
		Metrics: metrics,
	}
}

func (o *Observer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	if o.Tracer == nil {
		return ctx, noopSpan{}
	}
	return o.Tracer.StartSpan(ctx, name)
}

type noopSpan struct{}

func (n noopSpan) End()                                         {}
func (n noopSpan) AddEvent(name string, attrs ...Field)         {}
func (n noopSpan) SetAttribute(key string, value interface{})   {}
func (n noopSpan) RecordError(err error)                        {}

var _ ObserverInterface = (*Observer)(nil)

type ObserverInterface interface {
	StartSpan(ctx context.Context, name string) (context.Context, Span)
}

func IntField(key string, v int) Field           { return Field{Key: key, Value: v} }
func StrField(key string, v string) Field         { return Field{Key: key, Value: v} }
func FloatField(key string, v float64) Field      { return Field{Key: key, Value: v} }
func BoolField(key string, v bool) Field          { return Field{Key: key, Value: v} }
func DurationField(key string, v time.Duration) Field { return Field{Key: key, Value: v.String()} }
func ErrField(err error) Field                    { return Field{Key: "error", Value: err.Error()} }
