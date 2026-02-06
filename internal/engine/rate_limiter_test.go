package engine

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_BasicLimit(t *testing.T) {
	limiter := NewRateLimiter()
	limiter.SetLimit(NodeTypeHTTPRequest, 2, 100*time.Millisecond)

	ctx := context.Background()

	if err := limiter.Wait(ctx, NodeTypeHTTPRequest); err != nil {
		t.Fatalf("first request should succeed: %v", err)
	}

	if err := limiter.Wait(ctx, NodeTypeHTTPRequest); err != nil {
		t.Fatalf("second request should succeed: %v", err)
	}

	start := time.Now()
	if err := limiter.Wait(ctx, NodeTypeHTTPRequest); err != nil {
		t.Fatalf("third request should succeed after wait: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 90*time.Millisecond {
		t.Errorf("expected wait time >= 90ms, got %v", elapsed)
	}
}

func TestRateLimiter_NoLimit(t *testing.T) {
	limiter := NewRateLimiter()
	ctx := context.Background()

	if err := limiter.Wait(ctx, NodeTypeCustomCode); err != nil {
		t.Fatalf("unlimited node should not block: %v", err)
	}
}

func TestRateLimiter_ContextCancel(t *testing.T) {
	limiter := NewRateLimiter()
	limiter.SetLimit(NodeTypeHTTPRequest, 1, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	if err := limiter.Wait(ctx, NodeTypeHTTPRequest); err != nil {
		t.Fatalf("first request should succeed: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := limiter.Wait(ctx, NodeTypeHTTPRequest); err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}
