package circuitbreaker

import (
	"fmt"
	"sync"
	"time"
)

type State int

const (
	CLOSED   State = iota
	OPEN
	HALF_OPEN
)

type CircuitBreaker struct {
	name             string
	state            State
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailure      time.Time
	mu               sync.RWMutex
}

func New(name string, failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		state:            CLOSED,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if cb.state == OPEN {
		if time.Since(cb.lastFailure) < cb.timeout {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker '%s' is OPEN", cb.name)
		}
		cb.state = HALF_OPEN
		cb.successCount = 0
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()
		if cb.failureCount >= cb.failureThreshold {
			cb.state = OPEN
		}
		return err
	}

	if cb.state == HALF_OPEN {
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = CLOSED
			cb.failureCount = 0
		}
	} else {
		cb.failureCount = 0
	}
	return nil
}
