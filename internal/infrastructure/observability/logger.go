package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type LoggerConfig struct {
	Level  Level
	Output io.Writer
}

func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:  LevelInfo,
		Output: os.Stdout,
	}
}

type structuredLogger struct {
	level  Level
	out    io.Writer
	fields []Field
	mu     sync.Mutex
}

func NewLogger(cfg LoggerConfig) Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	return &structuredLogger{
		level: cfg.Level,
		out:   cfg.Output,
	}
}

func (l *structuredLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, LevelDebug, msg, fields...)
}

func (l *structuredLogger) Info(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, LevelInfo, msg, fields...)
}

func (l *structuredLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, LevelWarn, msg, fields...)
}

func (l *structuredLogger) Error(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, LevelError, msg, fields...)
}

func (l *structuredLogger) With(fields ...Field) Logger {
	newLogger := &structuredLogger{
		level:  l.level,
		out:    l.out,
		fields: make([]Field, len(l.fields)+len(fields)),
	}
	copy(newLogger.fields, l.fields)
	copy(newLogger.fields[len(l.fields):], fields)
	return newLogger
}

func (l *structuredLogger) log(ctx context.Context, level Level, msg string, fields ...Field) {
	if level < l.level {
		return
	}

	entry := make(map[string]interface{}, 8)
	entry["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	entry["level"] = level.String()
	entry["message"] = msg

	for _, f := range l.fields {
		entry[f.Key] = f.Value
	}
	for _, f := range fields {
		entry[f.Key] = f.Value
	}
	for _, f := range ExtractCorrelationFields(ctx) {
		entry[f.Key] = f.Value
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: marshal error: %v\n", err)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.out, string(data))
}
