package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type ShutdownManager struct {
	hooks        []ShutdownHook
	mu           sync.RWMutex
	shutdownOnce sync.Once
	logger       *slog.Logger
}

type ShutdownHook struct {
	Name     string
	Priority int
	Timeout  time.Duration
	Fn       func(context.Context) error
}

func NewShutdownManager(logger *slog.Logger) *ShutdownManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &ShutdownManager{
		hooks:  make([]ShutdownHook, 0),
		logger: logger,
	}
}

func (sm *ShutdownManager) Register(hook ShutdownHook) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if hook.Timeout == 0 {
		hook.Timeout = 30 * time.Second
	}

	sm.hooks = append(sm.hooks, hook)
}

func (sm *ShutdownManager) Shutdown(ctx context.Context) error {
	var shutdownErr error

	sm.shutdownOnce.Do(func() {
		sm.logger.Info("graceful shutdown initiated")

		sm.mu.RLock()
		hooks := make([]ShutdownHook, len(sm.hooks))
		copy(hooks, sm.hooks)
		sm.mu.RUnlock()

		sm.sortHooksByPriority(hooks)

		for _, hook := range hooks {
			hookCtx, cancel := context.WithTimeout(ctx, hook.Timeout)

			sm.logger.Info("executing shutdown hook",
				slog.String("name", hook.Name),
				slog.Int("priority", hook.Priority),
			)

			if err := hook.Fn(hookCtx); err != nil {
				sm.logger.Error("shutdown hook failed",
					slog.String("name", hook.Name),
					slog.Any("error", err),
				)
				if shutdownErr == nil {
					shutdownErr = fmt.Errorf("hook %s failed: %w", hook.Name, err)
				}
			} else {
				sm.logger.Info("shutdown hook completed", slog.String("name", hook.Name))
			}

			cancel()
		}

		if shutdownErr == nil {
			sm.logger.Info("graceful shutdown completed")
		} else {
			sm.logger.Error("graceful shutdown completed with errors", slog.Any("error", shutdownErr))
		}
	})

	return shutdownErr
}

func (sm *ShutdownManager) sortHooksByPriority(hooks []ShutdownHook) {
	for i := 0; i < len(hooks); i++ {
		for j := i + 1; j < len(hooks); j++ {
			if hooks[i].Priority < hooks[j].Priority {
				hooks[i], hooks[j] = hooks[j], hooks[i]
			}
		}
	}
}

func (sm *ShutdownManager) WaitForSignal(ctx context.Context, signals <-chan struct{}) {
	select {
	case <-signals:
		sm.logger.Info("shutdown signal received")
		sm.Shutdown(ctx)
	case <-ctx.Done():
		sm.logger.Info("context cancelled")
		sm.Shutdown(context.Background())
	}
}
