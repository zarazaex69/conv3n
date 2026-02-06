package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// WorkerPool manages concurrent workflow executions with configurable limits

type WorkerPool struct {
	maxWorkers int
	semaphore  chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	active     int
}

// NewWorkerPool creates a new worker pool with the specified maximum workers

func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 100 // Default limit to prevent runaway resource usage
	}

	return &WorkerPool{
		maxWorkers: maxWorkers,
		semaphore:  make(chan struct{}, maxWorkers),
	}
}

// Execute runs a workflow execution function with concurrency control

func (wp *WorkerPool) Execute(ctx context.Context, fn func() error) error {

	select {
	case wp.semaphore <- struct{}{}:

	case <-ctx.Done():
		return ctx.Err()
	}

	wp.mu.Lock()
	wp.active++
	currentActive := wp.active
	wp.mu.Unlock()

	log.Printf("Worker pool: acquired slot (%d/%d active)", currentActive, wp.maxWorkers)

	wp.wg.Add(1)

	// Execute in goroutine
	go func() {
		defer func() {

			<-wp.semaphore

			wp.mu.Lock()
			wp.active--
			currentActive := wp.active
			wp.mu.Unlock()

			log.Printf("Worker pool: released slot (%d/%d active)", currentActive, wp.maxWorkers)

			wp.wg.Done()
		}()

		if err := fn(); err != nil {
			log.Printf("Worker pool: execution failed: %v", err)
		}
	}()

	return nil
}

// ExecuteSync runs a workflow execution function synchronously with concurrency control

func (wp *WorkerPool) ExecuteSync(ctx context.Context, fn func() error) error {

	select {
	case wp.semaphore <- struct{}{}:

	case <-ctx.Done():
		return ctx.Err()
	}

	defer func() {
		<-wp.semaphore
	}()

	wp.mu.Lock()
	wp.active++
	currentActive := wp.active
	wp.mu.Unlock()

	log.Printf("Worker pool: executing sync (%d/%d active)", currentActive, wp.maxWorkers)

	defer func() {
		wp.mu.Lock()
		wp.active--
		wp.mu.Unlock()
	}()

	return fn()
}

// Wait blocks until all active workers complete

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
	log.Println("Worker pool: all workers completed")
}

// ActiveCount returns the current number of active workers
func (wp *WorkerPool) ActiveCount() int {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return wp.active
}

// Capacity returns the maximum number of workers
func (wp *WorkerPool) Capacity() int {
	return wp.maxWorkers
}

// Available returns the number of available slots
func (wp *WorkerPool) Available() int {
	return wp.maxWorkers - wp.ActiveCount()
}

// Stats returns current pool statistics
func (wp *WorkerPool) Stats() WorkerPoolStats {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	return WorkerPoolStats{
		MaxWorkers: wp.maxWorkers,
		Active:     wp.active,
		Available:  wp.maxWorkers - wp.active,
	}
}

// WorkerPoolStats contains worker pool statistics
type WorkerPoolStats struct {
	MaxWorkers int `json:"max_workers"`
	Active     int `json:"active"`
	Available  int `json:"available"`
}

// String returns a human-readable representation of the stats
func (s WorkerPoolStats) String() string {
	return fmt.Sprintf("WorkerPool[%d/%d active, %d available]", s.Active, s.MaxWorkers, s.Available)
}
