package aiplane

import "context"

type EventStore interface {
	Append(ctx context.Context, event interface{}) error
}
