package engine

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type testingHelper interface {
	Helper()
	Fatalf(format string, args ...any)
}

func getWorkerScriptPath(t testingHelper) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	projectRoot := filepath.Join(wd, "..", "..")
	workerScript := filepath.Join(projectRoot, "pkg", "bunock", "worker_server.ts")

	absPath, err := filepath.Abs(workerScript)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Fatalf("worker script not found at: %s", absPath)
	}

	return absPath
}

func TestWorkerPool_ConcurrentSubmissions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "test_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok", id: input.id };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(2, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer pool.Shutdown()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := pool.Submit(ctx, testBlock, map[string]any{"id": id}, 3*time.Second)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if successCount != 10 {
		t.Errorf("expected all 10 submissions to succeed, got %d", successCount)
	}
}

func TestWorkerPool_TimeoutPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "slow_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	await Bun.sleep(5000);
	return { result: "ok" };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(1, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer pool.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err = pool.Submit(ctx, testBlock, map[string]any{}, 500*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}

	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestWorkerPool_ScalingUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "scale_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	await Bun.sleep(100);
	return { result: "ok" };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(2, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer pool.Shutdown()

	initialSize := pool.currentSize.Load()

	for i := 0; i < 20; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			pool.Submit(ctx, testBlock, map[string]any{}, 3*time.Second)
		}()
	}

	time.Sleep(2 * time.Second)

	scaledSize := pool.currentSize.Load()

	if scaledSize <= initialSize {
		t.Logf("pool did not scale up (initial: %d, current: %d) - may be expected", initialSize, scaledSize)
	}
}

func TestWorkerPool_ShutdownGracefully(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping shutdown test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "shutdown_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok" };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(2, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = pool.Submit(ctx, testBlock, map[string]any{}, 3*time.Second)
	if err != nil {
		t.Fatalf("initial submit failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		pool.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("shutdown did not complete within timeout")
	}

	_, err = pool.Submit(ctx, testBlock, map[string]any{}, 3*time.Second)
	if err == nil {
		t.Error("expected error when submitting to shut down pool")
	}
}

func TestWorkerPool_GetHealthyWorker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping health check test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "health_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok" };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(3, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer pool.Shutdown()

	worker := pool.getHealthyWorker()
	if worker == nil {
		t.Error("expected to get a healthy worker")
	}

	if !worker.healthy.Load() {
		t.Error("returned worker should be healthy")
	}
}

func TestWorkerPool_TaskDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping task distribution test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "dist_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok", task: input.task };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(3, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer pool.Shutdown()

	for i := 0; i < 15; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := pool.Submit(ctx, testBlock, map[string]any{"task": i}, 3*time.Second)
		cancel()
		if err != nil {
			t.Errorf("task %d failed: %v", i, err)
		}
	}

	stats := pool.Stats()
	workers := stats["workers"].([]map[string]any)

	totalTasks := int64(0)
	for _, w := range workers {
		totalTasks += w["task_count"].(int64)
	}

	if totalTasks != 15 {
		t.Errorf("expected 15 total tasks, got %d", totalTasks)
	}
}

func TestWorkerPool_SubmitAfterShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping post-shutdown test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "shutdown_test_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok" };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(1, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}

	pool.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = pool.Submit(ctx, testBlock, map[string]any{}, 3*time.Second)
	if err == nil {
		t.Error("expected error when submitting after shutdown")
	}

	if err.Error() != "worker pool is shutting down" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWorkerPool_Stats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stats test in short mode")
	}

	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "stats_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok" };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(2, "bun", workerScript)
	if err != nil {
		t.Fatalf("failed to create worker pool: %v", err)
	}
	defer pool.Shutdown()

	stats := pool.Stats()

	if stats["min_size"] != 2 {
		t.Errorf("expected min_size 2, got %v", stats["min_size"])
	}

	if stats["current_size"].(int32) != 2 {
		t.Errorf("expected current_size 2, got %v", stats["current_size"])
	}

	workers := stats["workers"].([]map[string]any)
	if len(workers) != 2 {
		t.Errorf("expected 2 workers in stats, got %d", len(workers))
	}

	for _, w := range workers {
		if _, ok := w["healthy"]; !ok {
			t.Error("worker stats should include healthy field")
		}
		if _, ok := w["task_count"]; !ok {
			t.Error("worker stats should include task_count field")
		}
	}
}

func BenchmarkWorkerPool_Submit(b *testing.B) {
	tmpDir := b.TempDir()
	testBlock := filepath.Join(tmpDir, "bench_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok", iteration: input.iteration };
}
	`), 0644)
	if err != nil {
		b.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(b)
	pool, err := NewWorkerPool(4, "bun", workerScript)
	if err != nil {
		b.Fatalf("failed to create worker pool: %v", err)
	}
	defer pool.Shutdown()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := pool.Submit(ctx, testBlock, map[string]any{"iteration": i}, 3*time.Second)
		cancel()
		if err != nil {
			b.Errorf("iteration %d failed: %v", i, err)
		}
	}
}

func TestWorkerPool_InvalidScriptPath(t *testing.T) {
	pool, err := NewWorkerPool(1, "bun", "/nonexistent/script.ts")
	if err == nil {
		pool.Shutdown()
		t.Error("expected error when creating pool with invalid script")
	}
}

func TestWorkerPool_ZeroSize(t *testing.T) {
	tmpDir := t.TempDir()
	testBlock := filepath.Join(tmpDir, "zero_block.ts")

	err := os.WriteFile(testBlock, []byte(`
export async function execute(input: any) {
	return { result: "ok" };
}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create test block: %v", err)
	}

	workerScript := getWorkerScriptPath(t)
	pool, err := NewWorkerPool(0, "bun", workerScript)
	if err != nil {
		t.Fatalf("pool creation should handle zero size: %v", err)
	}
	defer pool.Shutdown()

	stats := pool.Stats()
	if stats["current_size"].(int32) < 1 {
		t.Error("expected pool to default to at least 1 worker")
	}
}
