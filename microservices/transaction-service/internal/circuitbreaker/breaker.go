package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

type CircuitBreaker struct {
	failures     int
	threshold    int
	resetTimeout time.Duration
	state        string // "closed", "open", "half-open"
	resetTimer   *time.Timer
	mu           sync.Mutex
}

func New(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
		state:        "closed",
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if cb.state == "open" {
		cb.mu.Unlock()
		return errors.New("circuit breaker is open — service unavailable")
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.state = "open"
			if cb.resetTimer != nil {
				cb.resetTimer.Stop()
			}
			cb.resetTimer = time.AfterFunc(cb.resetTimeout, func() {
				cb.mu.Lock()
				cb.state = "half-open"
				cb.mu.Unlock()
			})
		}
		return err
	}

	// Success: reset failures
	cb.failures = 0
	cb.state = "closed"
	return nil
}
