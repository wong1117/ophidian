package rabbitmq

import (
	"context"
)

type Subscriber struct {
	// TODO: implement
}

func NewSubscriber(url string) *Subscriber {
	return &Subscriber{}
}

func (s *Subscriber) Subscribe(ctx context.Context, queue string, handler func([]byte) error) error {
	return nil
}
