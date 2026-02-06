package engine

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	t.Run("Execute_ConcurrencyLimit", func(t *testing.T) {
		maxWorkers := 2
		pool := NewWorkerPool(maxWorkers)

		var activeWorkers int32
		var maxActiveWorkers int32
		var wg sync.WaitGroup

		totalTasks := 10
		wg.Add(totalTasks)

		for i := 0; i < totalTasks; i++ {
			go func() {
				err := pool.Execute(context.Background(), func() error {
					current := atomic.AddInt32(&activeWorkers, 1)

					// Track maximum concurrent workers
					for {
						max := atomic.LoadInt32(&maxActiveWorkers)
						if current > max {
							if !atomic.CompareAndSwapInt32(&maxActiveWorkers, max, current) {
								continue
							}
						}
						break
					}

					time.Sleep(10 * time.Millisecond) // Simulate work
					atomic.AddInt32(&activeWorkers, -1)
					return nil
				})
				if err != nil {
					t.Errorf("execution failed: %v", err)
				}
				wg.Done()
			}()
		}

		wg.Wait()

		if maxActiveWorkers > int32(maxWorkers) {
			t.Errorf("expected max %d workers, got %d", maxWorkers, maxActiveWorkers)
		}
	})

	t.Run("Execute_ContextCancellation", func(t *testing.T) {
		pool := NewWorkerPool(1)

		// Fill the pool
		started := make(chan struct{})
		go pool.Execute(context.Background(), func() error {
			close(started)
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		<-started

		// Try to execute with cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := pool.Execute(ctx, func() error {
			return nil
		})

		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("ExecuteSync", func(t *testing.T) {
		pool := NewWorkerPool(1)

		executed := false
		err := pool.ExecuteSync(context.Background(), func() error {
			executed = true
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !executed {
			t.Error("function was not executed")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		pool := NewWorkerPool(5)
		stats := pool.Stats()

		if stats.MaxWorkers != 5 {
			t.Errorf("expected 5 max workers, got %d", stats.MaxWorkers)
		}
		if stats.Active != 0 {
			t.Errorf("expected 0 active workers, got %d", stats.Active)
		}
		if stats.Available != 5 {
			t.Errorf("expected 5 available workers, got %d", stats.Available)
		}
	})

	t.Run("ErrorPropagation", func(t *testing.T) {
		pool := NewWorkerPool(1)
		expectedErr := errors.New("task failed")

		// ExecuteSync should propagate error
		err := pool.ExecuteSync(context.Background(), func() error {
			return expectedErr
		})

		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}

		// Execute (async) logs error but doesn't return it directly

		// but we can verify it doesn't panic
		err = pool.Execute(context.Background(), func() error {
			return expectedErr
		})
		if err != nil {
			t.Errorf("Execute should not return error from async task: %v", err)
		}
	})

	t.Run("Wait", func(t *testing.T) {
		pool := NewWorkerPool(2)
		var wg sync.WaitGroup
		wg.Add(2)

		// Start 2 workers that sleep
		for i := 0; i < 2; i++ {
			go pool.Execute(context.Background(), func() error {
				time.Sleep(50 * time.Millisecond)
				wg.Done()
				return nil
			})
		}

		// Wait for them to be scheduled

		time.Sleep(10 * time.Millisecond)

		// Wait for pool to drain
		pool.Wait()

		if pool.ActiveCount() != 0 {
			t.Errorf("expected 0 active workers, got %d", pool.ActiveCount())
		}
	})
}
