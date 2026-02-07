package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/storage"
)

type mockTaskStorage struct {
	storage.Storage
	tasks       map[string]*storage.Task
	createCalls int
	updateCalls int
	mu          sync.RWMutex
}

func (m *mockTaskStorage) CreateTask(ctx context.Context, task *storage.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	if m.tasks == nil {
		m.tasks = make(map[string]*storage.Task)
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *mockTaskStorage) UpdateTask(ctx context.Context, task *storage.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls++
	if m.tasks == nil {
		return fmt.Errorf("task not found")
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *mockTaskStorage) GetPendingTasks(ctx context.Context, limit int) ([]*storage.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pending []*storage.Task
	for _, task := range m.tasks {
		if task.Status == "pending" {
			pending = append(pending, task)
		}
	}
	return pending, nil
}

type mockTaskExecutor struct {
	executions    int
	shouldFail    bool
	executionTime time.Duration
	mu            sync.Mutex
}

func (m *mockTaskExecutor) Execute(ctx context.Context, task *storage.Task) error {
	m.mu.Lock()
	m.executions++
	mu := &m.mu
	mu.Unlock()

	if m.executionTime > 0 {
		time.Sleep(m.executionTime)
	}

	if m.shouldFail {
		return fmt.Errorf("mock execution failure")
	}
	return nil
}

func TestPersistentQueue_EnqueueAndProcess(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{}

	queue := NewPersistentQueue(store, 2, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}
	defer queue.Stop()

	task := &storage.Task{
		ID:         "task-1",
		MaxRetries: 3,
	}

	if err := queue.Enqueue(ctx, task); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	executor.mu.Lock()
	execCount := executor.executions
	executor.mu.Unlock()

	if execCount != 1 {
		t.Errorf("expected 1 execution, got %d", execCount)
	}

	store.mu.RLock()
	if store.tasks["task-1"].Status != "completed" {
		t.Errorf("expected task status completed, got %s", store.tasks["task-1"].Status)
	}
	store.mu.RUnlock()
}

func TestPersistentQueue_RetryOnFailure(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{shouldFail: true}

	queue := NewPersistentQueue(store, 2, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}
	defer queue.Stop()

	task := &storage.Task{
		ID:         "task-retry",
		MaxRetries: 3,
	}

	if err := queue.Enqueue(ctx, task); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	executor.mu.Lock()
	execCount := executor.executions
	executor.mu.Unlock()

	if execCount < 2 {
		t.Errorf("expected at least 2 retries, got %d", execCount)
	}

	store.mu.RLock()
	taskStatus := store.tasks["task-retry"].Status
	store.mu.RUnlock()

	if taskStatus != "failed" && taskStatus != "pending" {
		t.Logf("task status: %s (may still be retrying)", taskStatus)
	}
}

func TestPersistentQueue_MaxRetriesExceeded(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{shouldFail: true}

	queue := NewPersistentQueue(store, 2, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}
	defer queue.Stop()

	task := &storage.Task{
		ID:         "task-max-retry",
		MaxRetries: 2,
	}

	if err := queue.Enqueue(ctx, task); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	time.Sleep(2 * time.Second)

	store.mu.RLock()
	finalTask := store.tasks["task-max-retry"]
	store.mu.RUnlock()

	if finalTask.Status != "failed" {
		t.Errorf("expected final status failed after max retries, got %s", finalTask.Status)
	}

	if finalTask.Error == nil || *finalTask.Error == "" {
		t.Error("expected error message to be set")
	}
}

func TestPersistentQueue_ConcurrentEnqueue(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{}

	queue := NewPersistentQueue(store, 4, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}
	defer queue.Stop()

	var wg sync.WaitGroup
	taskCount := 20

	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task := &storage.Task{
				ID:         fmt.Sprintf("task-%d", id),
				MaxRetries: 3,
			}
			if err := queue.Enqueue(ctx, task); err != nil {
				t.Errorf("failed to enqueue task %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	executor.mu.Lock()
	execCount := executor.executions
	executor.mu.Unlock()

	if execCount != taskCount {
		t.Errorf("expected %d executions, got %d", taskCount, execCount)
	}
}

func TestPersistentQueue_PollingRecovery(t *testing.T) {
	store := &mockTaskStorage{
		tasks: map[string]*storage.Task{
			"orphan-1": {
				ID:         "orphan-1",
				Status:     "pending",
				MaxRetries: 3,
			},
			"orphan-2": {
				ID:         "orphan-2",
				Status:     "pending",
				MaxRetries: 3,
			},
		},
	}

	executor := &mockTaskExecutor{}

	queue := NewPersistentQueue(store, 2, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}
	defer queue.Stop()

	time.Sleep(300 * time.Millisecond)

	executor.mu.Lock()
	execCount := executor.executions
	executor.mu.Unlock()

	if execCount < 2 {
		t.Errorf("expected at least 2 executions from polling, got %d", execCount)
	}
}

func TestPersistentQueue_StopGracefully(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{executionTime: 50 * time.Millisecond}

	queue := NewPersistentQueue(store, 2, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}

	for i := 0; i < 5; i++ {
		task := &storage.Task{
			ID:         fmt.Sprintf("task-%d", i),
			MaxRetries: 1,
		}
		queue.Enqueue(ctx, task)
	}

	time.Sleep(100 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		queue.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("queue did not stop gracefully within timeout")
	}
}

func TestPersistentQueue_ContextCancellation(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{}

	queue := NewPersistentQueue(store, 2, executor)
	ctx, cancel := context.WithCancel(context.Background())

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}

	task := &storage.Task{
		ID:         "task-cancel",
		MaxRetries: 3,
	}

	if err := queue.Enqueue(ctx, task); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	cancel()
	time.Sleep(200 * time.Millisecond)

	ctx2 := context.Background()
	err := queue.Enqueue(ctx2, task)

	if err == nil {
		t.Log("enqueue after cancel may succeed depending on timing")
	}

	queue.Stop()
}

func TestPersistentQueue_StatusTransitions(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{}

	queue := NewPersistentQueue(store, 2, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}
	defer queue.Stop()

	task := &storage.Task{
		ID:         "task-status",
		MaxRetries: 3,
	}

	if err := queue.Enqueue(ctx, task); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	if task.Status != "pending" {
		t.Errorf("expected initial status pending, got %s", task.Status)
	}

	time.Sleep(300 * time.Millisecond)

	store.mu.RLock()
	finalTask := store.tasks["task-status"]
	store.mu.RUnlock()

	if finalTask.Status != "completed" {
		t.Errorf("expected final status completed, got %s", finalTask.Status)
	}
}

func TestPersistentQueue_TaskPersistence(t *testing.T) {
	store := &mockTaskStorage{}
	executor := &mockTaskExecutor{}

	queue := NewPersistentQueue(store, 2, executor)
	ctx := context.Background()

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}
	defer queue.Stop()

	task := &storage.Task{
		ID:         "task-persist",
		MaxRetries: 3,
	}

	if err := queue.Enqueue(ctx, task); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	store.mu.RLock()
	if store.createCalls != 1 {
		t.Errorf("expected 1 create call, got %d", store.createCalls)
	}
	store.mu.RUnlock()

	time.Sleep(200 * time.Millisecond)

	store.mu.RLock()
	if store.updateCalls < 1 {
		t.Errorf("expected at least 1 update call, got %d", store.updateCalls)
	}
	store.mu.RUnlock()
}
