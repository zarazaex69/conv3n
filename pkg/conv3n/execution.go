package conv3n

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type ExecutionHandle struct {
	id       string
	ctx      *engine.ExecutionContext
	storage  storage.Storage
	registry *engine.ExecutionRegistry
	events   EventHandler
}

type ExecutionState string

const (
	ExecutionStatePending   ExecutionState = "pending"
	ExecutionStateRunning   ExecutionState = "running"
	ExecutionStateCompleted ExecutionState = "completed"
	ExecutionStateFailed    ExecutionState = "failed"
	ExecutionStateCancelled ExecutionState = "cancelled"
)

type ExecutionStatus struct {
	ID          string
	WorkflowID  string
	Status      ExecutionState
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       *string
}

func (h *ExecutionHandle) ID() string {
	return h.id
}

func (h *ExecutionHandle) Stop() error {
	if err := h.registry.Stop(h.id); err != nil {
		return fmt.Errorf("failed to stop execution: %w", err)
	}

	if h.events != nil {
		h.events.OnExecutionStop(h.id)
	}

	return nil
}

func (h *ExecutionHandle) Status(ctx context.Context) (*ExecutionStatus, error) {
	exec, err := h.storage.GetExecution(ctx, h.id)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution status: %w", err)
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

func (h *ExecutionHandle) GetNodeResult(ctx context.Context, nodeID string) (map[string]interface{}, error) {
	resultBytes, err := h.storage.GetNodeResult(ctx, h.id, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node result: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse node result: %w", err)
	}

	return result, nil
}

func (h *ExecutionHandle) GetState(ctx context.Context) (map[string]interface{}, error) {
	exec, err := h.storage.GetExecution(ctx, h.id)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(exec.State, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return state, nil
}

func (h *ExecutionHandle) Wait(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := h.Status(ctx)
			if err != nil {
				return err
			}

			switch status.Status {
			case ExecutionStateCompleted:
				if h.events != nil {
					h.events.OnExecutionComplete(h.id, nil)
				}
				return nil
			case ExecutionStateFailed:
				if h.events != nil {
					errMsg := ""
					if status.Error != nil {
						errMsg = *status.Error
					}
					h.events.OnExecutionComplete(h.id, fmt.Errorf("%s", errMsg))
				}
				return fmt.Errorf("execution failed: %s", *status.Error)
			case ExecutionStateCancelled:
				return ErrExecutionCancelled
			}
		}
	}
}
