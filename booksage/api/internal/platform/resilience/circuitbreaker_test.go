package resilience

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if cb.CurrentState() != StateClosed {
		t.Errorf("Expected Closed, got %d", cb.CurrentState())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	testErr := errors.New("fail")

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return testErr })
	}

	if cb.CurrentState() != StateOpen {
		t.Errorf("Expected Open after 3 failures, got %d", cb.CurrentState())
	}

	// Should reject calls when open
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	testErr := errors.New("fail")

	// Trip the breaker
	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })

	if cb.CurrentState() != StateOpen {
		t.Fatalf("Expected Open, got %d", cb.CurrentState())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Next call should transition to HalfOpen and succeed
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("Expected success in HalfOpen, got %v", err)
	}

	if cb.CurrentState() != StateClosed {
		t.Errorf("Expected Closed after successful HalfOpen call, got %d", cb.CurrentState())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	testErr := errors.New("fail")

	// Trip
	_ = cb.Execute(func() error { return testErr })

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Fail again in HalfOpen → should go back to Open
	_ = cb.Execute(func() error { return testErr })

	if cb.CurrentState() != StateOpen {
		t.Errorf("Expected Open after HalfOpen failure, got %d", cb.CurrentState())
	}
}
