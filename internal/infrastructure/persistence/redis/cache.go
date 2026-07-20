package redis

import (
	"context"
	"encoding/json"
	"time"
	"github.com/redis/go-redis/v9"
)

type CacheStore struct {
	client *redis.Client
}

func NewCacheStore(client *redis.Client) *CacheStore {
	return &CacheStore{client: client}
}

func (s *CacheStore) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (s *CacheStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, data, ttl).Err()
}

func (s *CacheStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
