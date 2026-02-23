package resilience

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing, reject calls
	StateHalfOpen              // Testing if service recovered
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker implements the circuit breaker pattern.
// Transitions: Closed → Open (after failThreshold consecutive failures)
//
//	Open → HalfOpen (after openTimeout expires)
//	HalfOpen → Closed (on success) or Open (on failure)
type CircuitBreaker struct {
	mu            sync.Mutex
	state         State
	failCount     int
	failThreshold int
	openTimeout   time.Duration
	openedAt      time.Time
}

// NewCircuitBreaker creates a circuit breaker with the given thresholds.
func NewCircuitBreaker(failThreshold int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:         StateClosed,
		failThreshold: failThreshold,
		openTimeout:   openTimeout,
	}
}

// Execute runs fn through the circuit breaker.
// Returns ErrCircuitOpen if the circuit is open and the timeout hasn't elapsed.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	switch cb.state {
	case StateOpen:
		if time.Since(cb.openedAt) > cb.openTimeout {
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			return cb.tryCall(fn)
		}
		cb.mu.Unlock()
		return ErrCircuitOpen

	case StateHalfOpen:
		cb.mu.Unlock()
		return cb.tryCall(fn)

	default: // Closed
		cb.mu.Unlock()
		return cb.tryCall(fn)
	}
}

func (cb *CircuitBreaker) tryCall(fn func() error) error {
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failCount++
		if cb.failCount >= cb.failThreshold {
			cb.state = StateOpen
			cb.openedAt = time.Now()
		}
		return err
	}

	// Success: reset
	cb.failCount = 0
	cb.state = StateClosed
	return nil
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) CurrentState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
