package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type CircuitState int32

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

type CircuitBreakerConfig struct {
	MaxFailures     uint32
	Timeout         time.Duration
	HalfOpenMaxReqs uint32
	OnStateChange   func(from, to CircuitState)
}

func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:     5,
		Timeout:         60 * time.Second,
		HalfOpenMaxReqs: 1,
	}
}

type CircuitBreaker struct {
	cfg           *CircuitBreakerConfig
	state         atomic.Int32
	failures      atomic.Uint32
	lastFailTime  atomic.Int64
	halfOpenReqs  atomic.Uint32
	mu            sync.RWMutex
	successCount  atomic.Uint64
	failureCount  atomic.Uint64
	rejectedCount atomic.Uint64
}

func NewCircuitBreaker(cfg *CircuitBreakerConfig) *CircuitBreaker {
	if cfg == nil {
		cfg = DefaultCircuitBreakerConfig()
	}

	cb := &CircuitBreaker{
		cfg: cfg,
	}
	cb.state.Store(int32(StateClosed))

	return cb
}

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() (any, error)) (any, error) {
	if err := cb.beforeRequest(); err != nil {
		cb.rejectedCount.Add(1)
		return nil, err
	}

	result, err := fn()

	cb.afterRequest(err)

	return result, err
}

func (cb *CircuitBreaker) beforeRequest() error {
	state := CircuitState(cb.state.Load())

	switch state {
	case StateClosed:
		return nil

	case StateOpen:
		lastFail := time.Unix(0, cb.lastFailTime.Load())
		if time.Since(lastFail) > cb.cfg.Timeout {
			cb.setState(StateHalfOpen)
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		if cb.halfOpenReqs.Add(1) > cb.cfg.HalfOpenMaxReqs {
			cb.halfOpenReqs.Add(^uint32(0))
			return ErrTooManyRequests
		}
		return nil

	default:
		return nil
	}
}

func (cb *CircuitBreaker) afterRequest(err error) {
	state := CircuitState(cb.state.Load())

	if err == nil {
		cb.onSuccess(state)
	} else {
		cb.onFailure(state)
	}
}

func (cb *CircuitBreaker) onSuccess(state CircuitState) {
	cb.successCount.Add(1)

	switch state {
	case StateClosed:
		cb.failures.Store(0)

	case StateHalfOpen:
		cb.setState(StateClosed)
		cb.failures.Store(0)
		cb.halfOpenReqs.Store(0)
	}
}

func (cb *CircuitBreaker) onFailure(state CircuitState) {
	cb.failureCount.Add(1)
	cb.lastFailTime.Store(time.Now().UnixNano())

	switch state {
	case StateClosed:
		failures := cb.failures.Add(1)
		if failures >= cb.cfg.MaxFailures {
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		cb.setState(StateOpen)
		cb.halfOpenReqs.Store(0)
	}
}

func (cb *CircuitBreaker) setState(newState CircuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := CircuitState(cb.state.Load())
	if oldState == newState {
		return
	}

	cb.state.Store(int32(newState))

	if cb.cfg.OnStateChange != nil {
		cb.cfg.OnStateChange(oldState, newState)
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	return CircuitState(cb.state.Load())
}

func (cb *CircuitBreaker) Stats() map[string]any {
	return map[string]any{
		"state":          cb.State().String(),
		"failures":       cb.failures.Load(),
		"success_count":  cb.successCount.Load(),
		"failure_count":  cb.failureCount.Load(),
		"rejected_count": cb.rejectedCount.Load(),
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state.Store(int32(StateClosed))
	cb.failures.Store(0)
	cb.halfOpenReqs.Store(0)
	cb.successCount.Store(0)
	cb.failureCount.Store(0)
	cb.rejectedCount.Store(0)
}

type CircuitBreakerRegistry struct {
	breakers sync.Map
	cfg      *CircuitBreakerConfig
}

func NewCircuitBreakerRegistry(cfg *CircuitBreakerConfig) *CircuitBreakerRegistry {
	if cfg == nil {
		cfg = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreakerRegistry{
		cfg: cfg,
	}
}

func (r *CircuitBreakerRegistry) Get(key string) *CircuitBreaker {
	if cb, ok := r.breakers.Load(key); ok {
		return cb.(*CircuitBreaker)
	}

	cb := NewCircuitBreaker(r.cfg)
	actual, _ := r.breakers.LoadOrStore(key, cb)
	return actual.(*CircuitBreaker)
}

func (r *CircuitBreakerRegistry) GetForURL(nodeType NodeType, url string) *CircuitBreaker {
	key := fmt.Sprintf("%s:%s", nodeType, url)
	return r.Get(key)
}

func (r *CircuitBreakerRegistry) Execute(ctx context.Context, key string, fn func() (any, error)) (any, error) {
	cb := r.Get(key)
	return cb.Execute(ctx, fn)
}

func (r *CircuitBreakerRegistry) Stats() map[string]any {
	stats := make(map[string]any)

	r.breakers.Range(func(key, value any) bool {
		cb := value.(*CircuitBreaker)
		stats[key.(string)] = cb.Stats()
		return true
	})

	return stats
}
