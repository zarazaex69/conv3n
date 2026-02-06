package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	ctx := context.Background()
	result, err := cb.Execute(ctx, func() (any, error) {
		return "success", nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result != "success" {
		t.Fatalf("expected 'success', got %v", result)
	}

	if cb.State() != StateClosed {
		t.Fatalf("expected state closed, got %v", cb.State())
	}
}

func TestCircuitBreakerOpens(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.MaxFailures = 3
	cb := NewCircuitBreaker(cfg)

	ctx := context.Background()
	testErr := errors.New("test error")

	for i := 0; i < 3; i++ {
		cb.Execute(ctx, func() (any, error) {
			return nil, testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state open, got %v", cb.State())
	}

	_, err := cb.Execute(ctx, func() (any, error) {
		return "should not execute", nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.MaxFailures = 2
	cfg.Timeout = 100 * time.Millisecond
	cb := NewCircuitBreaker(cfg)

	ctx := context.Background()
	testErr := errors.New("test error")

	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() (any, error) {
			return nil, testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state open, got %v", cb.State())
	}

	time.Sleep(150 * time.Millisecond)

	result, err := cb.Execute(ctx, func() (any, error) {
		return "recovered", nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result != "recovered" {
		t.Fatalf("expected 'recovered', got %v", result)
	}

	if cb.State() != StateClosed {
		t.Fatalf("expected state closed after recovery, got %v", cb.State())
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	ctx := context.Background()

	cb.Execute(ctx, func() (any, error) {
		return "success", nil
	})

	cb.Execute(ctx, func() (any, error) {
		return nil, errors.New("fail")
	})

	stats := cb.Stats()

	if stats["success_count"].(uint64) != 1 {
		t.Fatalf("expected 1 success, got %v", stats["success_count"])
	}

	if stats["failure_count"].(uint64) != 1 {
		t.Fatalf("expected 1 failure, got %v", stats["failure_count"])
	}
}
