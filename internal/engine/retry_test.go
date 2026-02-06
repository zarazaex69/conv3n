package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetrySuccess(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.MaxAttempts = 3
	cfg.InitialBackoff = 10 * time.Millisecond

	ctx := context.Background()
	attempts := 0

	result, err := RetryWithBackoff(ctx, cfg, func(ctx context.Context) (any, error) {
		attempts++
		if attempts < 2 {
			return nil, errors.New("temporary failure")
		}
		return "success", nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result != "success" {
		t.Fatalf("expected 'success', got %v", result)
	}

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryMaxAttempts(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.MaxAttempts = 3
	cfg.InitialBackoff = 10 * time.Millisecond

	ctx := context.Background()
	attempts := 0

	_, err := RetryWithBackoff(ctx, cfg, func(ctx context.Context) (any, error) {
		attempts++
		return nil, errors.New("timeout")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryNonRetryableError(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.MaxAttempts = 3
	cfg.RetryableErrors = []string{"timeout"}

	ctx := context.Background()
	attempts := 0

	_, err := RetryWithBackoff(ctx, cfg, func(ctx context.Context) (any, error) {
		attempts++
		return nil, errors.New("fatal error")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts != 1 {
		t.Fatalf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetryContextCancellation(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.MaxAttempts = 10
	cfg.InitialBackoff = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0

	_, err := RetryWithBackoff(ctx, cfg, func(ctx context.Context) (any, error) {
		attempts++
		return nil, errors.New("timeout")
	})

	if err != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	if attempts > 2 {
		t.Fatalf("expected few attempts due to context cancellation, got %d", attempts)
	}
}
