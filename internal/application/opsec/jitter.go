package opsec

import (
	"context"
	"math/rand"
	"time"
)

type JitterConfig struct {
	Enabled         bool
	MinDelay        time.Duration
	MaxDelay        time.Duration
	SleepMasking    bool
	SleepMaskType   SleepMaskType
	TimingRandomize bool
}

type SleepMaskType string

const (
	SleepMaskNone         SleepMaskType = "none"
	SleepMaskAPI          SleepMaskType = "api_call"
	SleepMaskCalculations SleepMaskType = "calculations"
	SleepMaskWaitable     SleepMaskType = "waitable_timer"
	SleepMaskEventLoop    SleepMaskType = "event_loop"
)

type JitterEngine struct {
	config JitterConfig
	rng    *rand.Rand
}

func NewJitterEngine(config JitterConfig) *JitterEngine {
	return &JitterEngine{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (e *JitterEngine) Sleep(ctx context.Context) error {
	if !e.config.Enabled {
		return nil
	}

	delay := e.NextDelay()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

func (e *JitterEngine) SleepWithMask(ctx context.Context) error {
	if !e.config.Enabled || !e.config.SleepMasking {
		return e.Sleep(ctx)
	}

	delay := e.NextDelay()
	endTime := time.Now().Add(delay)

	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		e.executeMaskCycle()
	}
	return nil
}

func (e *JitterEngine) NextDelay() time.Duration {
	if !e.config.Enabled {
		return 0
	}
	if e.config.MinDelay == 0 {
		e.config.MinDelay = 500 * time.Millisecond
	}
	if e.config.MaxDelay == 0 {
		e.config.MaxDelay = 5 * time.Second
	}
	return e.config.MinDelay + time.Duration(e.rng.Int63n(int64(e.config.MaxDelay-e.config.MinDelay)))
}

func (e *JitterEngine) RandomizedTiming(base time.Duration) time.Duration {
	if !e.config.TimingRandomize {
		return base
	}
	jitter := time.Duration(e.rng.Int63n(int64(base / 3)))
	if e.rng.Intn(2) == 0 {
		return base + jitter
	}
	return base - jitter
}

func (e *JitterEngine) executeMaskCycle() {
	switch e.config.SleepMaskType {
	case SleepMaskAPI:
		e.maskAPICall()
	case SleepMaskCalculations:
		e.maskCalculations()
	case SleepMaskWaitable:
		e.maskWaitableTimer()
	case SleepMaskEventLoop:
		e.maskEventLoop()
	default:
		e.maskAPICall()
	}
}

func (e *JitterEngine) maskAPICall() {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i * 31 % 256)
	}
	_ = data
}

func (e *JitterEngine) maskCalculations() {
	sum := 0
	for i := 0; i < 1000; i++ {
		sum += i * i
	}
	_ = sum
}

func (e *JitterEngine) maskWaitableTimer() {
	time.Sleep(10 * time.Millisecond)
}

func (e *JitterEngine) maskEventLoop() {
	for i := 0; i < 100; i++ {
		_ = i * i
	}
}
