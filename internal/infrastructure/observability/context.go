package observability

import (
	"context"
	"crypto/rand"
	"fmt"
)

type ctxKey string

const (
	correlationIDKey ctxKey = "correlation_id"
	requestIDKey     ctxKey = "request_id"
)

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

func CorrelationID(ctx context.Context) string {
	id, _ := ctx.Value(correlationIDKey).(string)
	return id
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func NewRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func NewCorrelationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func ExtractCorrelationFields(ctx context.Context) []Field {
	var fields []Field
	if cid := CorrelationID(ctx); cid != "" {
		fields = append(fields, StrField("correlation_id", cid))
	}
	if rid := RequestID(ctx); rid != "" {
		fields = append(fields, StrField("request_id", rid))
	}
	return fields
}
