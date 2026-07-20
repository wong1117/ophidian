package ha

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testChecker struct {
	name  string
	err   error
	delay time.Duration
}

func (t *testChecker) Name() string        { return t.name }
func (t *testChecker) Check(ctx context.Context) error {
	time.Sleep(t.delay)
	return t.err
}

func TestHealthChecker_AllHealthy(t *testing.T) {
	h := NewHealthChecker()
	h.Register(&testChecker{name: "db"})
	h.Register(&testChecker{name: "redis"})

	results := h.CheckAll(context.Background())

	assert.Len(t, results, 2)
	assert.True(t, results["db"].Healthy)
	assert.True(t, results["redis"].Healthy)
	assert.True(t, h.IsHealthy())
}

func TestHealthChecker_PartialFailure(t *testing.T) {
	h := NewHealthChecker()
	h.Register(&testChecker{name: "db"})
	h.Register(&testChecker{name: "redis", err: errors.New("connection refused")})

	results := h.CheckAll(context.Background())

	assert.False(t, results["redis"].Healthy)
	assert.Equal(t, "connection refused", results["redis"].Error)
	assert.False(t, h.IsHealthy())
}

func TestHealthChecker_Empty(t *testing.T) {
	h := NewHealthChecker()
	assert.False(t, h.IsHealthy())
}

func TestReadinessChecker(t *testing.T) {
	r := NewReadinessChecker()
	r.Register(&testChecker{name: "db"})

	err := r.Check(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup not complete")

	r.SetReady()
	err = r.Check(context.Background())
	assert.NoError(t, err)
}

func TestReadinessChecker_ProbeFails(t *testing.T) {
	r := NewReadinessChecker()
	r.Register(&testChecker{name: "db", err: errors.New("connection refused")})
	r.SetReady()

	err := r.Check(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness probe db")
}

func TestMemoryLeaderElection(t *testing.T) {
	e := NewMemoryLeaderElection()

	ctx, cancel := context.WithCancel(context.Background())
	resigned, err := e.Campaign(ctx)
	assert.NoError(t, err)
	assert.True(t, e.IsLeader())

	cancel()
	<-resigned
	assert.False(t, e.IsLeader())
}

func TestMemoryLeaderElection_Resign(t *testing.T) {
	e := NewMemoryLeaderElection()

	_, _ = e.Campaign(context.Background())
	assert.True(t, e.IsLeader())

	e.Resign(context.Background())
	assert.False(t, e.IsLeader())
}

func TestMemoryLeaderElection_Callback(t *testing.T) {
	e := NewMemoryLeaderElection()
	var status []bool
	e.OnLeadershipChange(func(isLeader bool) {
		status = append(status, isLeader)
	})

	ctx, cancel := context.WithCancel(context.Background())
	_, _ = e.Campaign(ctx)
	time.Sleep(10 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	assert.Len(t, status, 2)
	assert.True(t, status[0])
	assert.False(t, status[1])
}

func TestShutdownManager(t *testing.T) {
	var executed []string
	sm := NewShutdownManager(5 * time.Second)

	sm.AddHook(func() error { executed = append(executed, "hook1"); return nil })
	sm.AddHook(func() error { executed = append(executed, "hook2"); return nil })
	sm.AddHook(func() error { executed = append(executed, "hook3"); return nil })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := sm.Wait(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"hook3", "hook2", "hook1"}, executed, "hooks should execute in reverse order")
}

func TestShutdownManager_Timeout(t *testing.T) {
	sm := NewShutdownManager(10 * time.Millisecond)
	sm.AddHook(func() error { time.Sleep(time.Second); return nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sm.Wait(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestStartupManager_Success(t *testing.T) {
	var executed []string
	sm := NewStartupManager()
	sm.AddHook(func() error { executed = append(executed, "a"); return nil })
	sm.AddHook(func() error { executed = append(executed, "b"); return nil })

	err := sm.Run(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, executed)
}

func TestStartupManager_Failure(t *testing.T) {
	sm := NewStartupManager()
	sm.AddHook(func() error { return errors.New("db connection failed") })

	err := sm.Run(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup hook failed")
}

func TestRetry_Success(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = time.Millisecond

	attempts := 0
	err := Retry(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("fail")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_Exhausted(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.MaxAttempts = 2
	cfg.BaseDelay = time.Millisecond

	err := Retry(context.Background(), cfg, func() error {
		return errors.New("permanent failure")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry exhausted")
}

func TestRetry_Cancelled(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = time.Second

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, cfg, func() error { return errors.New("fail") })
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry cancelled")
}

func TestMetrics(t *testing.T) {
	m := NewMetrics()
	m.Increment("http.requests")
	m.Increment("http.requests")
	m.Increment("errors")

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap["http.requests"])
	assert.Equal(t, int64(1), snap["errors"])
}
