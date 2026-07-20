package ha

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type HealthStatus struct {
	Name      string    `json:"name"`
	Healthy   bool      `json:"healthy"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type HealthChecker struct {
	mu       sync.RWMutex
	checkers []Checker
	results  map[string]HealthStatus
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		results: make(map[string]HealthStatus),
	}
}

func (h *HealthChecker) Register(checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers = append(h.checkers, checker)
}

func (h *HealthChecker) CheckAll(ctx context.Context) map[string]HealthStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	results := make(map[string]HealthStatus, len(h.checkers))
	for _, c := range h.checkers {
		err := c.Check(ctx)
		status := HealthStatus{
			Name:      c.Name(),
			Healthy:   err == nil,
			CheckedAt: time.Now(),
		}
		if err != nil {
			status.Error = err.Error()
		}
		results[c.Name()] = status
		h.results[c.Name()] = status
	}
	return results
}

func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, s := range h.results {
		if !s.Healthy {
			return false
		}
	}
	return len(h.results) > 0
}

type ReadinessChecker struct {
	mu      sync.RWMutex
	ready   bool
	probes  []Checker
}

func NewReadinessChecker() *ReadinessChecker {
	return &ReadinessChecker{}
}

func (r *ReadinessChecker) Register(probe Checker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.probes = append(r.probes, probe)
}

func (r *ReadinessChecker) SetReady() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = true
}

func (r *ReadinessChecker) Check(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.ready {
		return fmt.Errorf("not ready: startup not complete")
	}
	for _, p := range r.probes {
		if err := p.Check(ctx); err != nil {
			return fmt.Errorf("readiness probe %s: %w", p.Name(), err)
		}
	}
	return nil
}

type LeaderElection interface {
	Campaign(ctx context.Context) (<-chan struct{}, error)
	IsLeader() bool
	Resign(ctx context.Context) error
}

type MemoryLeaderElection struct {
	mu       sync.Mutex
	leader   bool
	leaderCh chan struct{}
	callback func(isLeader bool)
}

func NewMemoryLeaderElection() *MemoryLeaderElection {
	return &MemoryLeaderElection{
		leaderCh: make(chan struct{}, 1),
	}
}

func (m *MemoryLeaderElection) Campaign(ctx context.Context) (<-chan struct{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	resigned := make(chan struct{})

	if !m.leader {
		m.leader = true
		m.leaderCh <- struct{}{}
		if m.callback != nil {
			m.callback(true)
		}
	}

	go func() {
		select {
		case <-ctx.Done():
		case <-m.leaderCh:
		}
		m.mu.Lock()
		m.leader = false
		m.mu.Unlock()
		close(resigned)
		if m.callback != nil {
			m.callback(false)
		}
	}()

	return resigned, nil
}

func (m *MemoryLeaderElection) IsLeader() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.leader
}

func (m *MemoryLeaderElection) Resign(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.leader {
		select {
		case <-m.leaderCh:
		default:
		}
		m.leader = false
	}
	return nil
}

func (m *MemoryLeaderElection) OnLeadershipChange(fn func(isLeader bool)) {
	m.callback = fn
}

type ShutdownManager struct {
	timeout  time.Duration
	hooks    []func() error
	log      func(string)
}

func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	return &ShutdownManager{
		timeout: timeout,
	}
}

func (s *ShutdownManager) WithLogger(logger func(string)) *ShutdownManager {
	s.log = logger
	return s
}

func (s *ShutdownManager) AddHook(hook func() error) {
	s.hooks = append(s.hooks, hook)
}

func (s *ShutdownManager) Wait(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		s.logf("received signal: %v", sig)
	case <-ctx.Done():
		s.logf("context cancelled")
	}

	return s.executeHooks()
}

func (s *ShutdownManager) executeHooks() error {
	done := make(chan struct{})
	var mu sync.Mutex
	var errs []error

	go func() {
		defer close(done)
		for i := len(s.hooks) - 1; i >= 0; i-- {
			if err := s.hooks[i](); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}
	}()

	select {
	case <-done:
		if len(errs) > 0 {
			return fmt.Errorf("shutdown errors: %v", errs)
		}
		return nil
	case <-time.After(s.timeout):
		return fmt.Errorf("shutdown timed out after %v", s.timeout)
	}
}

func (s *ShutdownManager) logf(format string, args ...interface{}) {
	if s.log != nil {
		s.log(fmt.Sprintf(format, args...))
	}
}

type StartupManager struct {
	hooks []func() error
	log   func(string)
}

func NewStartupManager() *StartupManager {
	return &StartupManager{}
}

func (s *StartupManager) WithLogger(logger func(string)) *StartupManager {
	s.log = logger
	return s
}

func (s *StartupManager) AddHook(hook func() error) {
	s.hooks = append(s.hooks, hook)
}

func (s *StartupManager) Run(ctx context.Context) error {
	for _, hook := range s.hooks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := hook(); err != nil {
			return fmt.Errorf("startup hook failed: %w", err)
		}
	}
	return nil
}

func (s *StartupManager) logf(format string, args ...interface{}) {
	if s.log != nil {
		s.log(fmt.Sprintf(format, args...))
	}
}

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Jitter:      0.2,
	}
}

func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if attempt == cfg.MaxAttempts {
				break
			}

			backoff := float64(cfg.BaseDelay) * math.Pow(2, float64(attempt-1))
			if backoff > float64(cfg.MaxDelay) {
				backoff = float64(cfg.MaxDelay)
			}
			backoff += backoff * cfg.Jitter * (rng.Float64()*2 - 1)

			select {
			case <-time.After(time.Duration(backoff)):
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("retry exhausted after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

type Metrics struct {
	mu       sync.Mutex
	counters map[string]int64
}

func NewMetrics() *Metrics {
	return &Metrics{counters: make(map[string]int64)}
}

func (m *Metrics) Increment(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name]++
}

func (m *Metrics) Snapshot() map[string]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make(map[string]int64, len(m.counters))
	for k, v := range m.counters {
		cp[k] = v
	}
	return cp
}
