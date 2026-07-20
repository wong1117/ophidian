package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
)

type PubSub struct {
	client *redis.Client
}

func NewPubSub(client *redis.Client) *PubSub {
	return &PubSub{client: client}
}

func (ps *PubSub) Publish(ctx context.Context, channel string, message []byte) error {
	return ps.client.Publish(ctx, channel, message).Err()
}

func (ps *PubSub) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return ps.client.Subscribe(ctx, channel)
}
