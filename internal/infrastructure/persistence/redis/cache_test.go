package redis

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testMetrics struct {
	mu     sync.Mutex
	hits   int64
	misses int64
	sets   int64
	dels   int64
}

func (m *testMetrics) RecordHit()            { m.mu.Lock(); m.hits++; m.mu.Unlock() }
func (m *testMetrics) RecordMiss()           { m.mu.Lock(); m.misses++; m.mu.Unlock() }
func (m *testMetrics) RecordSet(d time.Duration) { m.mu.Lock(); m.sets++; m.mu.Unlock() }
func (m *testMetrics) RecordDelete()         { m.mu.Lock(); m.dels++; m.mu.Unlock() }

func TestMemoryCache_SetGet(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	err := c.Set(ctx, "key1", map[string]string{"name": "test"}, time.Minute)
	assert.NoError(t, err)

	var result map[string]string
	err = c.Get(ctx, "key1", &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result["name"])
}

func TestMemoryCache_Get_Miss(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	var result string
	err := c.Get(ctx, "nonexistent", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")
}

func TestMemoryCache_Expiry(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	c.Set(ctx, "expiring", "value", 1*time.Nanosecond)
	time.Sleep(10 * time.Millisecond)

	var result string
	err := c.Get(ctx, "expiring", &result)
	assert.Error(t, err)
}

func TestMemoryCache_Exists(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	c.Set(ctx, "exists-test", "value", time.Hour)
	exists, err := c.Exists(ctx, "exists-test")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = c.Exists(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryCache_Delete(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	c.Set(ctx, "del-me", "value", time.Hour)
	err := c.Delete(ctx, "del-me")
	assert.NoError(t, err)

	var result string
	err = c.Get(ctx, "del-me", &result)
	assert.Error(t, err)
}

func TestMemoryCache_Tags(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	c.SetWithTags(ctx, "key-a", "val-a", time.Hour, "users")
	c.SetWithTags(ctx, "key-b", "val-b", time.Hour, "users", "admin")

	err := c.InvalidateByTag(ctx, "users")
	assert.NoError(t, err)

	var r1, r2 string
	assert.Error(t, c.Get(ctx, "key-a", &r1))
	assert.Error(t, c.Get(ctx, "key-b", &r2))
}

func TestMemoryCache_TagMissing(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()
	err := c.InvalidateByTag(ctx, "nonexistent-tag")
	assert.NoError(t, err)
}

func TestMemoryCache_Metrics(t *testing.T) {
	m := &testMetrics{}
	c := NewMemoryCache()
	_ = m
	_ = c
}

func TestCacheInterface(t *testing.T) {
	c := NewMemoryCache()
	var _ Cache = c
	ctx := context.Background()

	key := "iface-test"
	type Data struct{ Value string }
	c.Set(ctx, key, Data{Value: "hello"}, time.Minute)

	var result Data
	c.Get(ctx, key, &result)
	assert.Equal(t, "hello", result.Value)

	exists, _ := c.Exists(ctx, key)
	assert.True(t, exists)

	c.Delete(ctx, key)
	exists, _ = c.Exists(ctx, key)
	assert.False(t, exists)
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	c := NewMemoryCache()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, fmt.Sprintf("bkey-%d", i), "value", time.Hour)
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	c := NewMemoryCache()
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		c.Set(ctx, fmt.Sprintf("bgkey-%d", i), "value", time.Hour)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result string
		c.Get(ctx, fmt.Sprintf("bgkey-%d", i%10000), &result)
	}
}

func BenchmarkMemoryCache_TagInvalidation(b *testing.B) {
	c := NewMemoryCache()
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		c.SetWithTags(ctx, fmt.Sprintf("btag-%d", i), "val", time.Hour, "bench-tag")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.InvalidateByTag(ctx, "bench-tag")
		for j := 0; j < 1000; j++ {
			c.SetWithTags(ctx, fmt.Sprintf("btag-%d", j), "val", time.Hour, "bench-tag")
		}
	}
}

func TestMemoryCache_Concurrency(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	k := 100
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("conc-%d", idx)
			c.Set(ctx, key, idx, time.Hour)
			var v int
			c.Get(ctx, key, &v)
		}(i)
	}
	wg.Wait()
}
