package conv3n

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/observability"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type RuntimeV2 struct {
	config          *ConfigV2
	storage         storage.Storage
	registry        *engine.ExecutionRegistry
	workerPool      *engine.WorkerPoolV2
	healthChecker   *engine.HealthChecker
	shutdownManager *engine.ShutdownManager
	metrics         *observability.Metrics
	tracer          *observability.Tracer
	logger          *observability.Logger
	mu              sync.RWMutex
	running         bool
}

type ConfigV2 struct {
	BlocksDir       string
	StoragePath     string
	WorkerPoolSize  int
	BunRuntimePath  string
	WorkerScript    string
	EventHandler    EventHandler
	EnableTriggers  bool
	LogLevel        slog.Level
	HealthCheckTTL  time.Duration
	ShutdownTimeout time.Duration
}

func DefaultConfigV2() *ConfigV2 {
	return &ConfigV2{
		BlocksDir:       "pkg/blocks",
		StoragePath:     "conv3n.db",
		WorkerPoolSize:  4,
		BunRuntimePath:  "bun",
		WorkerScript:    "pkg/bunock/worker_server.ts",
		EnableTriggers:  false,
		LogLevel:        slog.LevelInfo,
		HealthCheckTTL:  5 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

func NewV2(cfg *ConfigV2) (*RuntimeV2, error) {
	if cfg == nil {
		cfg = DefaultConfigV2()
	}

	logger := observability.NewLogger(cfg.LogLevel, os.Stdout)
	observability.SetLogger(logger)

	store, err := storage.NewSQLite(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	workerPool, err := engine.NewWorkerPoolV2(
		cfg.WorkerPoolSize,
		cfg.BunRuntimePath,
		cfg.WorkerScript,
	)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to initialize worker pool: %w", err)
	}

	registry := engine.NewExecutionRegistry()
	healthChecker := engine.NewHealthChecker(cfg.HealthCheckTTL)
	shutdownManager := engine.NewShutdownManager(logger.Logger)

	healthChecker.Register("worker_pool", engine.WorkerPoolHealthCheck(workerPool))

	rt := &RuntimeV2{
		config:          cfg,
		storage:         store,
		registry:        registry,
		workerPool:      workerPool,
		healthChecker:   healthChecker,
		shutdownManager: shutdownManager,
		metrics:         observability.GetMetrics(),
		tracer:          observability.GetTracer(),
		logger:          logger,
		running:         false,
	}

	rt.registerShutdownHooks()

	return rt, nil
}

func (r *RuntimeV2) registerShutdownHooks() {
	r.shutdownManager.Register(engine.ShutdownHook{
		Name:     "cancel_executions",
		Priority: 100,
		Timeout:  10 * time.Second,
		Fn: func(ctx context.Context) error {
			r.logger.Info("cancelling active executions")
			r.registry.CancelAll()
			return nil
		},
	})

	r.shutdownManager.Register(engine.ShutdownHook{
		Name:     "worker_pool",
		Priority: 90,
		Timeout:  15 * time.Second,
		Fn: func(ctx context.Context) error {
			r.logger.Info("shutting down worker pool")
			r.workerPool.Shutdown()
			return nil
		},
	})

	r.shutdownManager.Register(engine.ShutdownHook{
		Name:     "storage",
		Priority: 80,
		Timeout:  5 * time.Second,
		Fn: func(ctx context.Context) error {
			r.logger.Info("closing storage")
			return r.storage.Close()
		},
	})
}

func (r *RuntimeV2) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return ErrAlreadyRunning
	}

	r.logger.Info("runtime starting",
		slog.String("blocks_dir", r.config.BlocksDir),
		slog.Int("worker_pool_size", r.config.WorkerPoolSize),
	)

	r.running = true
	r.metrics.Gauge("runtime.status", nil).Set(1)

	r.logger.Info("runtime started successfully")
	return nil
}

func (r *RuntimeV2) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return ErrNotRunning
	}

	r.logger.Info("runtime stopping")

	shutdownCtx, cancel := context.WithTimeout(ctx, r.config.ShutdownTimeout)
	defer cancel()

	if err := r.shutdownManager.Shutdown(shutdownCtx); err != nil {
		r.logger.Error("shutdown completed with errors", slog.Any("error", err))
	}

	r.running = false
	r.metrics.Gauge("runtime.status", nil).Set(0)

	r.logger.Info("runtime stopped")
	return nil
}

func (r *RuntimeV2) Execute(ctx context.Context, wf *Workflow, input map[string]any) (*ExecutionHandle, error) {
	r.mu.RLock()
	if !r.running {
		r.mu.RUnlock()
		return nil, ErrNotRunning
	}
	r.mu.RUnlock()

	span, ctx := r.tracer.StartSpan(ctx, "runtime.execute")
	defer span.End()

	engineWf := wf.toEngine()

	execCtx := engine.NewExecutionContext(engineWf.ID)
	if input != nil {
		execCtx.TriggerData = input
	}

	runner := engine.NewGraphRunnerV2(execCtx, r.workerPool, r.storage, r.registry)

	execID := execCtx.ExecutionID
	if execID == "" {
		execID = fmt.Sprintf("exec-%d", time.Now().UnixNano())
		execCtx.ExecutionID = execID
	}

	handle := &ExecutionHandle{
		id:       execID,
		ctx:      execCtx,
		storage:  r.storage,
		registry: r.registry,
		events:   r.config.EventHandler,
	}

	execContext, cancel := context.WithCancel(ctx)
	r.registry.Register(execID, cancel)

	if r.config.EventHandler != nil {
		r.config.EventHandler.OnExecutionStart(execID, engineWf.ID)
	}

	go func() {
		defer r.registry.Unregister(execID)

		if err := runner.Run(execContext, *engineWf); err != nil {
			r.logger.Error("execution failed",
				slog.String("execution_id", execID),
				slog.Any("error", err),
			)
			if r.config.EventHandler != nil {
				r.config.EventHandler.OnExecutionComplete(execID, err)
			}
			span.SetStatus(observability.StatusCodeError, err.Error())
			return
		}

		if r.config.EventHandler != nil {
			r.config.EventHandler.OnExecutionComplete(execID, nil)
		}
		span.SetStatus(observability.StatusCodeOK, "")
	}()

	return handle, nil
}

func (r *RuntimeV2) Health(ctx context.Context) map[string]any {
	results := r.healthChecker.Check(ctx)
	overallStatus := r.healthChecker.OverallStatus(ctx)

	health := map[string]any{
		"status":     string(overallStatus),
		"timestamp":  time.Now().Format(time.RFC3339),
		"components": make(map[string]any),
	}

	for name, component := range results {
		health["components"].(map[string]any)[name] = map[string]any{
			"status":    string(component.Status),
			"message":   component.Message,
			"timestamp": component.Timestamp.Format(time.RFC3339),
			"metadata":  component.Metadata,
		}
	}

	return health
}

func (r *RuntimeV2) Metrics() map[string]any {
	return r.metrics.Snapshot()
}

func (r *RuntimeV2) Traces() []map[string]any {
	return r.tracer.ExportAll()
}

func (r *RuntimeV2) WaitForShutdown(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		r.logger.Info("received shutdown signal", slog.String("signal", sig.String()))
		r.Stop(ctx)
	case <-ctx.Done():
		r.logger.Info("context cancelled")
		r.Stop(context.Background())
	}
}

func (r *RuntimeV2) GetExecution(ctx context.Context, execID string) (*ExecutionStatus, error) {
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

func (r *RuntimeV2) ListExecutions(ctx context.Context, workflowID string, limit int) ([]*ExecutionStatus, error) {
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

func (r *RuntimeV2) Close() error {
	ctx := context.Background()
	return r.Stop(ctx)
}
