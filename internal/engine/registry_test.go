package engine_test

import (
	"context"
	"testing"

	"github.com/zarazaex69/conv3n/internal/engine"
)

func TestExecutionRegistry(t *testing.T) {
	registry := engine.NewExecutionRegistry()

	t.Run("RegisterAndUnregister", func(t *testing.T) {
		execID := "exec-1"
		_, cancel := context.WithCancel(context.Background())

		registry.Register(execID, cancel)

		if !registry.IsActive(execID) {
			t.Error("expected execution to be active")
		}

		if registry.ActiveCount() != 1 {
			t.Errorf("expected 1 active execution, got %d", registry.ActiveCount())
		}

		registry.Unregister(execID)

		if registry.IsActive(execID) {
			t.Error("expected execution to be inactive")
		}

		if registry.ActiveCount() != 0 {
			t.Errorf("expected 0 active executions, got %d", registry.ActiveCount())
		}

		// Cleanup to prevent context leak
		cancel()
	})

	t.Run("Cancel", func(t *testing.T) {
		execID := "exec-2"
		ctx, cancel := context.WithCancel(context.Background())

		registry.Register(execID, cancel)

		err := registry.Cancel(execID)
		if err != nil {
			t.Errorf("unexpected error cancelling execution: %v", err)
		}

		// Verify context is cancelled
		select {
		case <-ctx.Done():
			// OK
		default:
			t.Error("expected context to be cancelled")
		}

		// Verify removed from registry
		if registry.IsActive(execID) {
			t.Error("expected execution to be removed after cancel")
		}

		// Cancel non-existent
		err = registry.Cancel("non-existent")
		if err == nil {
			t.Error("expected error cancelling non-existent execution")
		}
	})

	t.Run("CancelAll", func(t *testing.T) {
		registry := engine.NewExecutionRegistry() // New instance

		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithCancel(context.Background())

		registry.Register("exec-3", cancel1)
		registry.Register("exec-4", cancel2)

		if registry.ActiveCount() != 2 {
			t.Errorf("expected 2 active executions, got %d", registry.ActiveCount())
		}

		registry.CancelAll()

		if registry.ActiveCount() != 0 {
			t.Errorf("expected 0 active executions, got %d", registry.ActiveCount())
		}

		// Verify contexts cancelled
		select {
		case <-ctx1.Done():
		default:
			t.Error("expected ctx1 to be cancelled")
		}
		select {
		case <-ctx2.Done():
		default:
			t.Error("expected ctx2 to be cancelled")
		}
	})
}
