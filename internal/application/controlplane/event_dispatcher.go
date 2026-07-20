package controlplane

import "context"

type EventDispatcher interface {
	Dispatch(ctx context.Context, event interface{}) error
}
