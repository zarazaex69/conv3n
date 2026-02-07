package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zarazaex69/conv3n/internal/storage"
)

type PersistentQueue struct {
	store      storage.Storage
	pending    chan *storage.Task
	workers    int
	wg         sync.WaitGroup
	stopChan   chan struct{}
	mu         sync.RWMutex
	pollTicker *time.Ticker
	executor   TaskExecutor
}

type TaskExecutor interface {
	Execute(ctx context.Context, task *storage.Task) error
}

func NewPersistentQueue(store storage.Storage, workers int, executor TaskExecutor) *PersistentQueue {
	return &PersistentQueue{
		store:      store,
		pending:    make(chan *storage.Task, workers*2),
		workers:    workers,
		stopChan:   make(chan struct{}),
		pollTicker: time.NewTicker(5 * time.Second),
		executor:   executor,
	}
}

func (pq *PersistentQueue) Start(ctx context.Context) error {
	if err := pq.loadPendingTasks(ctx); err != nil {
		return fmt.Errorf("failed to load pending tasks: %w", err)
	}

	for i := 0; i < pq.workers; i++ {
		pq.wg.Add(1)
		go pq.worker(ctx, i)
	}

	pq.wg.Add(1)
	go pq.poller(ctx)

	return nil
}

func (pq *PersistentQueue) Stop() {
	close(pq.stopChan)
	pq.pollTicker.Stop()
	pq.wg.Wait()
}

func (pq *PersistentQueue) Enqueue(ctx context.Context, task *storage.Task) error {
	task.Status = "pending"
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	if err := pq.store.CreateTask(ctx, task); err != nil {
		return fmt.Errorf("failed to persist task: %w", err)
	}

	select {
	case pq.pending <- task:
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

func (pq *PersistentQueue) worker(ctx context.Context, id int) {
	defer pq.wg.Done()

	for {
		select {
		case task := <-pq.pending:
			pq.processTask(ctx, task)
		case <-pq.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (pq *PersistentQueue) processTask(ctx context.Context, task *storage.Task) {
	task.Status = "running"
	task.UpdatedAt = time.Now()
	pq.store.UpdateTask(ctx, task)

	err := pq.executor.Execute(ctx, task)

	if err != nil {
		task.Attempts++
		if task.Attempts >= task.MaxRetries {
			task.Status = "failed"
			errMsg := err.Error()
			task.Error = &errMsg
		} else {
			task.Status = "pending"
			delay := time.Duration(task.Attempts) * 5 * time.Second
			time.Sleep(delay)
			pq.pending <- task
		}
	} else {
		task.Status = "completed"
	}

	task.UpdatedAt = time.Now()
	pq.store.UpdateTask(ctx, task)
}

func (pq *PersistentQueue) poller(ctx context.Context) {
	defer pq.wg.Done()

	for {
		select {
		case <-pq.pollTicker.C:
			pq.loadPendingTasks(ctx)
		case <-pq.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (pq *PersistentQueue) loadPendingTasks(ctx context.Context) error {
	tasks, err := pq.store.GetPendingTasks(ctx, 100)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		select {
		case pq.pending <- task:
		default:
		}
	}

	return nil
}
