package engine

import (
	"context"
	"fmt"
	"math"
	"time"
)

type RetryConfig struct {
	MaxAttempts     int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffMultiple float64
	RetryableErrors []string
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      10 * time.Second,
		BackoffMultiple: 2.0,
		RetryableErrors: []string{
			"timeout",
			"connection refused",
			"temporary failure",
			"rate limit",
		},
	}
}

type RetryableFunc func(ctx context.Context) (any, error)

func RetryWithBackoff(ctx context.Context, cfg *RetryConfig, fn RetryableFunc) (any, error) {
	var lastErr error
	backoff := cfg.InitialBackoff

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			backoff = time.Duration(float64(backoff) * cfg.BackoffMultiple)
			if backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !isRetryable(err, cfg.RetryableErrors) {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		if attempt < cfg.MaxAttempts-1 {
			continue
		}
	}

	return nil, fmt.Errorf("max retry attempts reached: %w", lastErr)
}

func isRetryable(err error, retryableErrors []string) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, pattern := range retryableErrors {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func ExponentialBackoff(attempt int, initial, max time.Duration) time.Duration {
	backoff := time.Duration(float64(initial) * math.Pow(2, float64(attempt)))
	if backoff > max {
		return max
	}
	return backoff
}
