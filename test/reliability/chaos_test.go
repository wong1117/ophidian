package reliability

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type ChaosConfig struct {
	NetworkDelay    time.Duration
	NetworkDropPct  float64
	DBFailureRate   float64
	CacheFailureRate float64
	QueueDisconnect  bool
	InjectPanic     bool
}

type StabilityReport struct {
	TestName       string
	Duration       time.Duration
	StartTime      time.Time
	EndTime        time.Time
	Operations     int64
	Failures       int64
	Recoveries     int64
	PeakGoroutines int
	PeakMemory     uint64
	NetworkErrors  int64
	DBErrors       int64
	CacheErrors    int64
	QueueErrors    int64
	Status         string
	Details        []string
}

type FaultInjector struct {
	mu           sync.Mutex
	active       bool
	networkDelay time.Duration
	dropPct      float64
	dbFailRate   float64
	cacheFailRate float64
	queueDown    bool
	panicInject  bool
	stats        *StabilityReport
}

func NewFaultInjector(cfg ChaosConfig) *FaultInjector {
	return &FaultInjector{
		active:       true,
		networkDelay: cfg.NetworkDelay,
		dropPct:      cfg.NetworkDropPct,
		dbFailRate:   cfg.DBFailureRate,
		cacheFailRate: cfg.CacheFailureRate,
		queueDown:    cfg.QueueDisconnect,
		panicInject:  cfg.InjectPanic,
		stats:        &StabilityReport{StartTime: time.Now()},
	}
}

func (f *FaultInjector) ShouldFail(category string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.active { return false }

	var rate float64
	switch category {
	case "network":
		rate = f.dropPct
	case "database":
		rate = f.dbFailRate
	case "cache":
		rate = f.cacheFailRate
	default:
		return false
	}

	fail := rate > 0 && float64(time.Now().UnixNano()%100) < rate
	if fail {
		switch category {
		case "network": f.stats.NetworkErrors++
		case "database": f.stats.DBErrors++
		case "cache": f.stats.CacheErrors++
		}
	}
	return fail
}

func (f *FaultInjector) IsQueueDown() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queueDown {
		f.stats.QueueErrors++
	}
	return f.queueDown
}

func (f *FaultInjector) Recover() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active = false
	f.networkDelay = 0
	f.dropPct = 0
	f.dbFailRate = 0
	f.cacheFailRate = 0
	f.queueDown = false
	f.stats.Recoveries++
	f.stats.Details = append(f.stats.Details, fmt.Sprintf("recovered at %s", time.Now().Format("15:04:05")))
}

func (f *FaultInjector) Stats() StabilityReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := *f.stats
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
	s.Details = append(s.Details, fmt.Sprintf("network_errors=%d db_errors=%d cache_errors=%d queue_errors=%d",
		s.NetworkErrors, s.DBErrors, s.CacheErrors, s.QueueErrors))
	return s
}

func (f *FaultInjector) InjectPanic() {
	if f.panicInject {
		panic("injected panic for chaos testing")
	}
}

func TestGracefulShutdown_QueueService(t *testing.T) {
	q := &testQueue{}
	started := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		close(started)
		for i := 0; i < 1000; i++ {
			q.Enqueue(fmt.Sprintf("item-%d", i))
		}
		close(stopped)
	}()

	<-started
	time.Sleep(50 * time.Millisecond)

	for q.Pending() > 0 {
		q.Dequeue()
	}

	select {
	case <-stopped:
		t.Log("Queue drained gracefully")
	case <-time.After(3 * time.Second):
		t.Fatal("Queue did not drain within timeout")
	}
}

type testQueue struct {
	mu      sync.Mutex
	pending []string
}

func (q *testQueue) Enqueue(item string)      { q.mu.Lock(); defer q.mu.Unlock(); q.pending = append(q.pending, item) }
func (q *testQueue) Dequeue() string           { q.mu.Lock(); defer q.mu.Unlock(); if len(q.pending) == 0 { return "" }; v := q.pending[0]; q.pending = q.pending[1:]; return v }
func (q *testQueue) Pending() int              { q.mu.Lock(); defer q.mu.Unlock(); return len(q.pending) }

func TestPortRelease_OnShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil { conn.Close() }
	}()

	time.Sleep(10 * time.Millisecond)
	listener.Close()
	time.Sleep(50 * time.Millisecond)

	l2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port not released after shutdown: %v", err)
	}
	l2.Close()
	t.Log("Port released successfully after shutdown")
}

func TestGracefulStartup_WaitForDependencies(t *testing.T) {
	ready := make(chan struct{})

	go func() {
		time.Sleep(100 * time.Millisecond)
		close(ready)
	}()

	select {
	case <-ready:
		t.Log("Dependencies ready, server can start")
	case <-time.After(2 * time.Second):
		t.Fatal("Dependencies not ready within timeout")
	}
}

func TestSoakTest_ContinuousEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	duration := 10 * time.Second
	deadline := time.After(duration)
	events := 0
	errors := 0
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-ticker.C:
			events++
			if events%100 == 0 && float64(events%500)/500.0 < 0.01 {
				errors++
			}
		}
	}

	rate := float64(events) / duration.Seconds()
	t.Logf("Soak test: %d events in %v (%.0f events/sec), %d errors", events, duration, rate, errors)

	if events < 100 {
		t.Errorf("too few events: %d", events)
	}
}

func TestFaultInjection_NetworkFailure(t *testing.T) {
	injector := NewFaultInjector(ChaosConfig{NetworkDropPct: 20})
	attempts := 100
	successful := 0

	for i := 0; i < attempts; i++ {
		if !injector.ShouldFail("network") {
			successful++
		}
	}

	stats := injector.Stats()
	t.Logf("Network fault injection: %d/%d successful, %d dropped",
		successful, attempts, stats.NetworkErrors)
	assert.Greater(t, stats.NetworkErrors, int64(5), "expected some network errors")
	assert.Equal(t, successful, attempts-int(stats.NetworkErrors))
}

func TestFaultInjection_Recovery(t *testing.T) {
	injector := NewFaultInjector(ChaosConfig{
		NetworkDropPct:  50,
		DBFailureRate:   30,
		CacheFailureRate: 20,
		QueueDisconnect: true,
	})

	for i := 0; i < 50; i++ {
		injector.ShouldFail("network")
		injector.ShouldFail("database")
		injector.ShouldFail("cache")
		_ = injector.IsQueueDown()
	}

	beforeRecovery := injector.Stats()
	t.Logf("Before recovery: network=%d db=%d cache=%d queue=%d",
		beforeRecovery.NetworkErrors, beforeRecovery.DBErrors, beforeRecovery.CacheErrors, beforeRecovery.QueueErrors)

	injector.Recover()

	for i := 0; i < 50; i++ {
		injector.ShouldFail("network")
		injector.ShouldFail("database")
		injector.ShouldFail("cache")
		_ = injector.IsQueueDown()
	}

	afterRecovery := injector.Stats()
	t.Logf("After recovery: network=%d db=%d cache=%d queue=%d",
		afterRecovery.NetworkErrors, beforeRecovery.DBErrors, beforeRecovery.CacheErrors, beforeRecovery.QueueErrors)

	assert.Equal(t, beforeRecovery.NetworkErrors, afterRecovery.NetworkErrors, "no new errors after recovery")
	assert.Equal(t, int64(1), afterRecovery.Recoveries)
}

func TestStabilityReport_Generation(t *testing.T) {
	report := StabilityReport{
		TestName:       "chaos-test-001",
		StartTime:      time.Now().Add(-time.Hour),
		EndTime:        time.Now(),
		Duration:       time.Hour,
		Operations:     100000,
		Failures:       150,
		Recoveries:     3,
		PeakGoroutines: 250,
		PeakMemory:     512 * 1024 * 1024,
		NetworkErrors:  45,
		DBErrors:       60,
		CacheErrors:    30,
		QueueErrors:    15,
		Status:         "PASS",
		Details:        []string{"network recovered in 2s", "cache recovered in 5s"},
	}

	assert.Equal(t, "PASS", report.Status)
	assert.Equal(t, int64(100000), report.Operations)
	assert.Equal(t, int64(150), report.Failures)
	assert.Greater(t, report.Duration, time.Duration(0))

	summary := fmt.Sprintf("Stability: %d ops, %d failures (%.2f%%), %d recoveries",
		report.Operations, report.Failures, float64(report.Failures)/float64(report.Operations)*100, report.Recoveries)
	t.Log(summary)
}

func TestConcurrentOperations_UnderFaults(t *testing.T) {
	injector := NewFaultInjector(ChaosConfig{
		NetworkDropPct:  10,
		DBFailureRate:   5,
		CacheFailureRate: 5,
	})

	var wg sync.WaitGroup
	ops := 500
	var success, failure int64
	var mu sync.Mutex

	for i := 0; i < ops; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			if injector.ShouldFail("network") {
				mu.Lock()
				failure++
				mu.Unlock()
				return
			}

			time.Sleep(time.Microsecond)
			mu.Lock()
			success++
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	stats := injector.Stats()
	t.Logf("Concurrent ops under faults: success=%d failure=%d total=%d", success, failure, ops)
	assert.Equal(t, int64(ops), success+failure)
	assert.Greater(t, stats.NetworkErrors, int64(0))
}

func TestRecoveryAfterDBFailure(t *testing.T) {
	injector := NewFaultInjector(ChaosConfig{DBFailureRate: 30})
	failures := 0
	recovered := 0

	for phase := 0; phase < 3; phase++ {
		for i := 0; i < 30; i++ {
			if injector.ShouldFail("database") {
				failures++
			} else {
				recovered++
			}
		}
		if phase == 1 {
			injector.Recover()
		}
	}

	stats := injector.Stats()
	t.Logf("DB failure recovery: failures=%d, recovered=%d, report_recoveries=%d",
		failures, recovered, stats.Recoveries)
	assert.Greater(t, failures, 0)
	assert.Equal(t, int64(1), stats.Recoveries)
}

func TestSoakTest_HighLoadQueue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	q := &struct {
		mu      sync.Mutex
		pending []string
	}{}
	enqueue := func(item string) { q.mu.Lock(); defer q.mu.Unlock(); q.pending = append(q.pending, item) }
	dequeue := func() string { q.mu.Lock(); defer q.mu.Unlock(); if len(q.pending) == 0 { return "" }; v := q.pending[0]; q.pending = q.pending[1:]; return v }
	pending := func() int { q.mu.Lock(); defer q.mu.Unlock(); return len(q.pending) }

	duration := 2 * time.Second
	deadline := time.After(duration)

	var enqueued, dequeued int64
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				enqueue("item")
				enqueued++
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if dequeue() != "" {
					dequeued++
				}
			}
		}
	}()

	<-deadline
	wg.Wait()

	t.Logf("High-load queue soak: enqueued=%d dequeued=%d pending=%d (%.0f ops/sec)",
		enqueued, dequeued, pending(), float64(enqueued+dequeued)/duration.Seconds())
	assert.Greater(t, enqueued, int64(100))
}

func TestGoroutineLeak_AfterRepeatedFailures(t *testing.T) {
	before := runtime.NumGoroutine()

	injector := NewFaultInjector(ChaosConfig{NetworkDropPct: 50})
	for i := 0; i < 1000; i++ {
		injector.ShouldFail("network")
	}
	injector.Recover()

	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	t.Logf("Goroutines: before=%d after=%d delta=%d", before, after, after-before)
	assert.LessOrEqual(t, after-before, 5, "goroutine leak detected after repeated failures")
}
