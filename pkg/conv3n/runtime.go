package conv3n

import (
	"context"
	"fmt"
	"sync"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type Runtime struct {
	config   *Config
	storage  storage.Storage
	registry *engine.ExecutionRegistry
	pool     *engine.WorkerPool
	mu       sync.RWMutex
	running  bool
}

type Config struct {
	BlocksDir      string
	StoragePath    string
	MaxWorkers     int
	EventHandler   EventHandler
	EnableTriggers bool
}

func DefaultConfig() *Config {
	return &Config{
		BlocksDir:      "pkg/blocks",
		StoragePath:    "conv3n.db",
		MaxWorkers:     10,
		EnableTriggers: false,
	}
}

func New(cfg *Config) (*Runtime, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	store, err := storage.NewSQLiteStorage(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	registry := engine.NewExecutionRegistry()
	pool := engine.NewWorkerPool(cfg.MaxWorkers)

	return &Runtime{
		config:   cfg,
		storage:  store,
		registry: registry,
		pool:     pool,
		running:  false,
	}, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return ErrAlreadyRunning
	}

	r.pool.Start(ctx)
	r.running = true

	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return ErrNotRunning
	}

	r.pool.Stop()
	r.registry.StopAll()
	r.running = false

	return nil
}

func (r *Runtime) Execute(ctx context.Context, wf *Workflow, input map[string]interface{}) (*ExecutionHandle, error) {
	r.mu.RLock()
	if !r.running {
		r.mu.RUnlock()
		return nil, ErrNotRunning
	}
	r.mu.RUnlock()

	engineWf := wf.toEngine()

	execCtx := engine.NewExecutionContext(engineWf.ID)
	if input != nil {
		execCtx.TriggerData = input
	}

	runner := engine.NewWorkflowRunner(execCtx, r.config.BlocksDir, r.storage, r.registry)
	if err := runner.LoadBlocks(); err != nil {
		return nil, fmt.Errorf("failed to load blocks: %w", err)
	}

	execID, err := r.storage.CreateExecution(ctx, engineWf.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	execCtx.ExecutionID = execID

	handle := &ExecutionHandle{
		id:       execID,
		ctx:      execCtx,
		storage:  r.storage,
		registry: r.registry,
		events:   r.config.EventHandler,
	}

	r.registry.Register(execID, execCtx)

	task := &engine.WorkflowTask{
		Workflow: engineWf,
		Runner:   runner,
		Context:  ctx,
	}

	r.pool.Submit(task)

	if r.config.EventHandler != nil {
		r.config.EventHandler.OnExecutionStart(execID, engineWf.ID)
	}

	return handle, nil
}

func (r *Runtime) GetExecution(ctx context.Context, execID string) (*ExecutionStatus, error) {
	exec, err := r.storage.GetExecution(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("execution not found: %w", err)
	}

	return &ExecutionStatus{
		ID:          exec.ID,
		WorkflowID:  exec.WorkflowID,
		Status:      ExecutionState(exec.Status),
		StartedAt:   exec.StartedAt,
		CompletedAt: exec.CompletedAt,
		Error:       exec.Error,
	}, nil
}

func (r *Runtime) ListExecutions(ctx context.Context, workflowID string, limit int) ([]*ExecutionStatus, error) {
	execs, err := r.storage.ListExecutions(ctx, workflowID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	result := make([]*ExecutionStatus, len(execs))
	for i, e := range execs {
		result[i] = &ExecutionStatus{
			ID:          e.ID,
			WorkflowID:  e.WorkflowID,
			Status:      ExecutionState(e.Status),
			StartedAt:   e.StartedAt,
			CompletedAt: e.CompletedAt,
			Error:       e.Error,
		}
	}

	return result, nil
}

func (r *Runtime) Close() error {
	ctx := context.Background()
	if err := r.Stop(ctx); err != nil && err != ErrNotRunning {
		return err
	}
	return r.storage.Close()
}
