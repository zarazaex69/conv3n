package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type RateLimiter struct {
	limiters map[NodeType]*tokenBucket
	mu       sync.RWMutex
}

type tokenBucket struct {
	tokens     int
	capacity   int
	refillRate time.Duration
	lastRefill time.Time
	mu         sync.Mutex
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[NodeType]*tokenBucket),
	}
}

func (rl *RateLimiter) SetLimit(nodeType NodeType, capacity int, refillRate time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.limiters[nodeType] = &tokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (rl *RateLimiter) Wait(ctx context.Context, nodeType NodeType) error {
	rl.mu.RLock()
	bucket, exists := rl.limiters[nodeType]
	rl.mu.RUnlock()

	if !exists {
		return nil
	}

	for {
		if rl.tryAcquire(bucket) {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("rate limit wait cancelled: %w", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (rl *RateLimiter) tryAcquire(bucket *tokenBucket) bool {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)

	if elapsed >= bucket.refillRate {
		refills := int(elapsed / bucket.refillRate)
		bucket.tokens = min(bucket.capacity, bucket.tokens+refills)
		bucket.lastRefill = bucket.lastRefill.Add(time.Duration(refills) * bucket.refillRate)
	}

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}
