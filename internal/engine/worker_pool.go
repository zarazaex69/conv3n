package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type WorkerPool struct {
	workers       []*BunWorker
	taskQueue     chan *WorkerTask
	resultQueue   chan *WorkerResult
	size          int
	runtimePath   string
	workerScript  string
	shutdownOnce  sync.Once
	shutdownChan  chan struct{}
	wg            sync.WaitGroup
	activeWorkers atomic.Int32
}

type WorkerTask struct {
	ID         string
	ScriptPath string
	Input      any
	ResultChan chan *WorkerResult
	Timeout    time.Duration
}

type WorkerResult struct {
	TaskID string
	Data   any
	Error  error
}

type BunWorker struct {
	id         int
	cmd        *exec.Cmd
	conn       net.Conn
	socketPath string
	encoder    *json.Encoder
	decoder    *json.Decoder
	mu         sync.Mutex
	healthy    atomic.Bool
	lastUsed   time.Time
	taskCount  atomic.Int64
}

func NewWorkerPool(size int, runtimePath, workerScript string) (*WorkerPool, error) {
	if size <= 0 {
		size = 4
	}

	pool := &WorkerPool{
		workers:      make([]*BunWorker, 0, size),
		taskQueue:    make(chan *WorkerTask, size*2),
		resultQueue:  make(chan *WorkerResult, size*2),
		size:         size,
		runtimePath:  runtimePath,
		workerScript: workerScript,
		shutdownChan: make(chan struct{}),
	}

	for i := 0; i < size; i++ {
		worker, err := pool.startWorker(i)
		if err != nil {
			pool.Shutdown()
			return nil, fmt.Errorf("failed to start worker %d: %w", i, err)
		}
		pool.workers = append(pool.workers, worker)
	}

	pool.wg.Add(1)
	go pool.dispatcher()

	return pool, nil
}

func (p *WorkerPool) startWorker(id int) (*BunWorker, error) {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("conv3n-worker-%d-%d.sock", os.Getpid(), id))

	if err := os.RemoveAll(socketPath); err != nil {
		return nil, fmt.Errorf("failed to remove old socket: %w", err)
	}

	cmd := exec.Command(p.runtimePath, "run", p.workerScript, socketPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start bun process: %w", err)
	}

	var conn net.Conn
	var dialErr error
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		conn, dialErr = net.Dial("unix", socketPath)
		if dialErr == nil {
			break
		}
	}

	if dialErr != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to connect to worker socket: %w", dialErr)
	}

	worker := &BunWorker{
		id:         id,
		cmd:        cmd,
		conn:       conn,
		socketPath: socketPath,
		encoder:    json.NewEncoder(conn),
		decoder:    json.NewDecoder(conn),
		lastUsed:   time.Now(),
	}

	worker.healthy.Store(true)
	p.activeWorkers.Add(1)

	return worker, nil
}

func (p *WorkerPool) dispatcher() {
	defer p.wg.Done()

	for {
		select {
		case task := <-p.taskQueue:
			p.wg.Add(1)
			go p.executeTask(task)
		case <-p.shutdownChan:
			return
		}
	}
}

func (p *WorkerPool) executeTask(task *WorkerTask) {
	defer p.wg.Done()

	worker := p.getHealthyWorker()
	if worker == nil {
		task.ResultChan <- &WorkerResult{
			TaskID: task.ID,
			Error:  fmt.Errorf("no healthy workers available"),
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), task.Timeout)
	defer cancel()

	resultChan := make(chan *WorkerResult, 1)

	go func() {
		result := worker.execute(task)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		task.ResultChan <- result
	case <-ctx.Done():
		worker.healthy.Store(false)
		task.ResultChan <- &WorkerResult{
			TaskID: task.ID,
			Error:  fmt.Errorf("task timeout after %v", task.Timeout),
		}
	}
}

func (p *WorkerPool) getHealthyWorker() *BunWorker {
	var bestWorker *BunWorker
	var minTasks int64 = -1

	for _, worker := range p.workers {
		if !worker.healthy.Load() {
			continue
		}

		tasks := worker.taskCount.Load()
		if minTasks == -1 || tasks < minTasks {
			minTasks = tasks
			bestWorker = worker
		}
	}

	return bestWorker
}

func (w *BunWorker) execute(task *WorkerTask) *WorkerResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.taskCount.Add(1)
	w.lastUsed = time.Now()

	request := map[string]any{
		"task_id":     task.ID,
		"script_path": task.ScriptPath,
		"input":       task.Input,
	}

	if err := w.encoder.Encode(request); err != nil {
		w.healthy.Store(false)
		return &WorkerResult{
			TaskID: task.ID,
			Error:  fmt.Errorf("failed to send task: %w", err),
		}
	}

	var response map[string]any
	if err := w.decoder.Decode(&response); err != nil {
		if err == io.EOF {
			w.healthy.Store(false)
			return &WorkerResult{
				TaskID: task.ID,
				Error:  fmt.Errorf("worker died unexpectedly"),
			}
		}
		return &WorkerResult{
			TaskID: task.ID,
			Error:  fmt.Errorf("failed to decode response: %w", err),
		}
	}

	if errMsg, hasErr := response["error"]; hasErr {
		return &WorkerResult{
			TaskID: task.ID,
			Error:  fmt.Errorf("%v", errMsg),
		}
	}

	return &WorkerResult{
		TaskID: task.ID,
		Data:   response["data"],
	}
}

func (p *WorkerPool) Submit(ctx context.Context, scriptPath string, input any, timeout time.Duration) (any, error) {
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

	task := &WorkerTask{
		ID:         taskID,
		ScriptPath: scriptPath,
		Input:      input,
		ResultChan: make(chan *WorkerResult, 1),
		Timeout:    timeout,
	}

	select {
	case p.taskQueue <- task:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.shutdownChan:
		return nil, fmt.Errorf("worker pool is shutting down")
	}

	select {
	case result := <-task.ResultChan:
		if result.Error != nil {
			return nil, result.Error
		}
		return result.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *WorkerPool) Shutdown() {
	p.shutdownOnce.Do(func() {
		close(p.shutdownChan)
		p.wg.Wait()

		for _, worker := range p.workers {
			worker.shutdown()
		}
	})
}

func (w *BunWorker) shutdown() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		w.conn.Close()
	}

	if w.cmd != nil && w.cmd.Process != nil {
		syscall.Kill(-w.cmd.Process.Pid, syscall.SIGTERM)
		time.AfterFunc(5*time.Second, func() {
			if w.cmd.Process != nil {
				w.cmd.Process.Kill()
			}
		})
		w.cmd.Wait()
	}

	os.RemoveAll(w.socketPath)
}

func (p *WorkerPool) Stats() map[string]any {
	stats := map[string]any{
		"pool_size":      p.size,
		"active_workers": p.activeWorkers.Load(),
		"queue_length":   len(p.taskQueue),
		"workers":        make([]map[string]any, 0, len(p.workers)),
	}

	for _, worker := range p.workers {
		workerStats := map[string]any{
			"id":         worker.id,
			"healthy":    worker.healthy.Load(),
			"task_count": worker.taskCount.Load(),
			"last_used":  worker.lastUsed.Format(time.RFC3339),
		}
		stats["workers"] = append(stats["workers"].([]map[string]any), workerStats)
	}

	return stats
}
