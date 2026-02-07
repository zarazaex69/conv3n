package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/zarazaex69/conv3n/internal/observability"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type CleanupJob struct {
	store     storage.Storage
	interval  time.Duration
	retention time.Duration
	stopChan  chan struct{}
	logger    *observability.Logger
}

func NewCleanupJob(store storage.Storage, interval, retention time.Duration) *CleanupJob {
	return &CleanupJob{
		store:     store,
		interval:  interval,
		retention: retention,
		stopChan:  make(chan struct{}),
		logger:    observability.GetLogger(),
	}
}

func (cj *CleanupJob) Start(ctx context.Context) {
	ticker := time.NewTicker(cj.interval)
	defer ticker.Stop()

	cj.logger.Info("cleanup job started",
		slog.Duration("interval", cj.interval),
		slog.Duration("retention", cj.retention),
	)

	for {
		select {
		case <-ticker.C:
			if err := cj.run(ctx); err != nil {
				cj.logger.Error("cleanup job failed", slog.Any("error", err))
			}
		case <-cj.stopChan:
			cj.logger.Info("cleanup job stopped")
			return
		case <-ctx.Done():
			cj.logger.Info("cleanup job cancelled")
			return
		}
	}
}

func (cj *CleanupJob) Stop() {
	close(cj.stopChan)
}

func (cj *CleanupJob) run(ctx context.Context) error {
	start := time.Now()

	if err := cj.store.CleanupOldExecutions(ctx, cj.retention); err != nil {
		return err
	}

	duration := time.Since(start)
	cj.logger.Info("cleanup completed", slog.Duration("duration", duration))

	return nil
}
