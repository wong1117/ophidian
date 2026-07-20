package rabbitmq

import (
	"context"
)

type Publisher struct {
	// TODO: implement
}

func NewPublisher(url string) *Publisher {
	return &Publisher{}
}

func (p *Publisher) Publish(ctx context.Context, exchange, key string, data []byte) error {
	return nil
}
