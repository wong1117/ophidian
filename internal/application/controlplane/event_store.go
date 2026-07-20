package controlplane

import "context"

type EventStore interface {
	Append(ctx context.Context, event interface{}) error
	Replay(ctx context.Context, aggregateID string) ([]interface{}, error)
}
