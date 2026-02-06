package engine

import (
	"context"
	"fmt"
	"sync"
)

// ExecutionRegistry manages active workflow executions

type ExecutionRegistry struct {
	mu       sync.RWMutex
	contexts map[string]context.CancelFunc
}

// NewExecutionRegistry creates a new registry for tracking active executions
func NewExecutionRegistry() *ExecutionRegistry {
	return &ExecutionRegistry{
		contexts: make(map[string]context.CancelFunc),
	}
}

// Register adds an execution to the registry with its cancel function

func (r *ExecutionRegistry) Register(execID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contexts[execID] = cancel
}

// Unregister removes an execution from the registry

func (r *ExecutionRegistry) Unregister(execID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.contexts, execID)
}

// Cancel stops a running execution by calling its context cancel function

func (r *ExecutionRegistry) Cancel(execID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cancel, exists := r.contexts[execID]
	if !exists {
		return fmt.Errorf("execution not found or already completed: %s", execID)
	}

	// Call cancel function to stop the execution
	cancel()

	// Remove from registry
	delete(r.contexts, execID)

	return nil
}

// IsActive checks if an execution is currently running
func (r *ExecutionRegistry) IsActive(execID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.contexts[execID]
	return exists
}

// ActiveCount returns the number of currently active executions
func (r *ExecutionRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.contexts)
}

// CancelAll stops all active executions

func (r *ExecutionRegistry) CancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for execID, cancel := range r.contexts {
		cancel()
		delete(r.contexts, execID)
	}
}
