package engine

import (
	"context"
	"sync"
	"time"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type ComponentHealth struct {
	Name      string
	Status    HealthStatus
	Message   string
	Timestamp time.Time
	Metadata  map[string]any
}

type HealthChecker struct {
	components map[string]HealthCheckFunc
	mu         sync.RWMutex
	cache      map[string]*ComponentHealth
	cacheTTL   time.Duration
}

type HealthCheckFunc func(ctx context.Context) *ComponentHealth

func NewHealthChecker(cacheTTL time.Duration) *HealthChecker {
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Second
	}

	return &HealthChecker{
		components: make(map[string]HealthCheckFunc),
		cache:      make(map[string]*ComponentHealth),
		cacheTTL:   cacheTTL,
	}
}

func (hc *HealthChecker) Register(name string, checkFunc HealthCheckFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.components[name] = checkFunc
}

func (hc *HealthChecker) Check(ctx context.Context) map[string]*ComponentHealth {
	hc.mu.RLock()
	components := make(map[string]HealthCheckFunc, len(hc.components))
	for name, fn := range hc.components {
		components[name] = fn
	}
	hc.mu.RUnlock()

	results := make(map[string]*ComponentHealth)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checkFunc := range components {
		if cached := hc.getCached(name); cached != nil {
			results[name] = cached
			continue
		}

		wg.Add(1)
		go func(n string, fn HealthCheckFunc) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			health := fn(checkCtx)
			if health == nil {
				health = &ComponentHealth{
					Name:      n,
					Status:    HealthStatusUnhealthy,
					Message:   "check returned nil",
					Timestamp: time.Now(),
				}
			}

			mu.Lock()
			results[n] = health
			hc.setCached(n, health)
			mu.Unlock()
		}(name, checkFunc)
	}

	wg.Wait()
	return results
}

func (hc *HealthChecker) getCached(name string) *ComponentHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if cached, ok := hc.cache[name]; ok {
		if time.Since(cached.Timestamp) < hc.cacheTTL {
			return cached
		}
	}
	return nil
}

func (hc *HealthChecker) setCached(name string, health *ComponentHealth) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.cache[name] = health
}

func (hc *HealthChecker) OverallStatus(ctx context.Context) HealthStatus {
	results := hc.Check(ctx)

	hasUnhealthy := false
	hasDegraded := false

	for _, health := range results {
		switch health.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return HealthStatusUnhealthy
	}
	if hasDegraded {
		return HealthStatusDegraded
	}
	return HealthStatusHealthy
}

func StorageHealthCheck(storage interface{ Ping(context.Context) error }) HealthCheckFunc {
	return func(ctx context.Context) *ComponentHealth {
		health := &ComponentHealth{
			Name:      "storage",
			Timestamp: time.Now(),
		}

		if err := storage.Ping(ctx); err != nil {
			health.Status = HealthStatusUnhealthy
			health.Message = err.Error()
		} else {
			health.Status = HealthStatusHealthy
			health.Message = "storage is accessible"
		}

		return health
	}
}

func WorkerPoolHealthCheck(pool *WorkerPoolV2) HealthCheckFunc {
	return func(ctx context.Context) *ComponentHealth {
		health := &ComponentHealth{
			Name:      "worker_pool",
			Timestamp: time.Now(),
		}

		stats := pool.Stats()
		activeWorkers := stats["active_workers"].(int32)
		poolSize := stats["pool_size"].(int)

		if activeWorkers == 0 {
			health.Status = HealthStatusUnhealthy
			health.Message = "no active workers"
		} else if activeWorkers < int32(poolSize)/2 {
			health.Status = HealthStatusDegraded
			health.Message = "less than 50% workers active"
		} else {
			health.Status = HealthStatusHealthy
			health.Message = "worker pool operational"
		}

		health.Metadata = stats

		return health
	}
}
