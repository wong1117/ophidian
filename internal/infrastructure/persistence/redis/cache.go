package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	InvalidateByTag(ctx context.Context, tag string) error
}

type CacheMetrics interface {
	RecordHit()
	RecordMiss()
	RecordSet(d time.Duration)
	RecordDelete()
}

type CacheStore struct {
	client  *redis.Client
	metrics CacheMetrics
}

func NewCacheStore(client *redis.Client) *CacheStore {
	return &CacheStore{client: client}
}

func (s *CacheStore) WithMetrics(m CacheMetrics) *CacheStore {
	s.metrics = m
	return s
}

func (s *CacheStore) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		s.recordMiss()
		if err == redis.Nil {
			return fmt.Errorf("cache miss: %s", key)
		}
		return fmt.Errorf("cache get %s: %w", key, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("cache unmarshal %s: %w", key, err)
	}
	s.recordHit()
	return nil
}

func (s *CacheStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal %s: %w", key, err)
	}
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set %s: %w", key, err)
	}
	s.recordSet(time.Since(start))
	return nil
}

func (s *CacheStore) SetWithTags(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error {
	if err := s.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	for _, tag := range tags {
		tagKey := "tag:" + tag
		if err := s.client.SAdd(ctx, tagKey, key).Err(); err != nil {
			return fmt.Errorf("cache tag %s for %s: %w", tag, key, err)
		}
		s.client.Expire(ctx, tagKey, ttl+time.Hour)
	}
	return nil
}

func (s *CacheStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache delete %s: %w", key, err)
	}
	s.recordDelete()
	return nil
}

func (s *CacheStore) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("cache exists %s: %w", key, err)
	}
	return n > 0, nil
}

func (s *CacheStore) InvalidateByTag(ctx context.Context, tag string) error {
	tagKey := "tag:" + tag
	members, err := s.client.SMembers(ctx, tagKey).Result()
	if err != nil {
		return fmt.Errorf("cache invalidate tag %s: %w", tag, err)
	}
	if len(members) == 0 {
		return nil
	}
	keys := append(members, tagKey)
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache invalidate tag delete %s: %w", tag, err)
	}
	return nil
}

func (s *CacheStore) recordHit() {
	if s.metrics != nil {
		s.metrics.RecordHit()
	}
}

func (s *CacheStore) recordMiss() {
	if s.metrics != nil {
		s.metrics.RecordMiss()
	}
}

func (s *CacheStore) recordSet(d time.Duration) {
	if s.metrics != nil {
		s.metrics.RecordSet(d)
	}
}

func (s *CacheStore) recordDelete() {
	if s.metrics != nil {
		s.metrics.RecordDelete()
	}
}

type MemoryCache struct {
	mu   sync.Mutex
	data map[string]cacheEntry
	tags map[string]map[string]struct{}
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]cacheEntry),
		tags: make(map[string]map[string]struct{}),
	}
}

func (c *MemoryCache) Get(ctx context.Context, key string, dest interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.data[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return fmt.Errorf("cache miss: %s", key)
	}
	data, _ := json.Marshal(entry.value)
	return json.Unmarshal(data, dest)
}

func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry{value: value, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (c *MemoryCache) SetWithTags(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry{value: value, expiresAt: time.Now().Add(ttl)}
	for _, tag := range tags {
		if c.tags[tag] == nil {
			c.tags[tag] = make(map[string]struct{})
		}
		c.tags[tag][key] = struct{}{}
	}
	return nil
}

func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.data[key]
	if !ok {
		return false, nil
	}
	return time.Now().Before(entry.expiresAt), nil
}

func (c *MemoryCache) InvalidateByTag(ctx context.Context, tag string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	keys, ok := c.tags[tag]
	if !ok {
		return nil
	}
	for key := range keys {
		delete(c.data, key)
	}
	delete(c.tags, tag)
	return nil
}
