package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

// LifecycleHandler handles execution lifecycle operations (stop, restart)
type LifecycleHandler struct {
	Store         storage.Storage
	Registry      *engine.ExecutionRegistry
	WorkerPool    *engine.WorkerPool
	BlockRegistry *engine.BlockRegistry
}

// NewLifecycleHandler creates a new lifecycle handler
func NewLifecycleHandler(store storage.Storage, registry *engine.ExecutionRegistry, workerPool *engine.WorkerPool, blockRegistry *engine.BlockRegistry) *LifecycleHandler {
	return &LifecycleHandler{
		Store:         store,
		Registry:      registry,
		WorkerPool:    workerPool,
		BlockRegistry: blockRegistry,
	}
}

// StopExecution handles POST /api/executions/{id}/stop

func (h *LifecycleHandler) StopExecution(w http.ResponseWriter, r *http.Request) {
	execID := r.PathValue("id")
	if execID == "" {
		http.Error(w, "Missing execution ID", http.StatusBadRequest)
		return
	}

	// Check if execution exists
	exec, err := h.Store.GetExecution(r.Context(), execID)
	if err != nil {
		http.Error(w, "Execution not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Check if execution is still running
	if exec.Status != storage.ExecutionStatusRunning {
		http.Error(w, fmt.Sprintf("Execution is not running (status: %s)", exec.Status), http.StatusBadRequest)
		return
	}

	// Cancel the execution via registry
	if err := h.Registry.Cancel(execID); err != nil {

		http.Error(w, "Failed to stop execution: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update status in database

	msg := "Execution stopped by user"
	if err := h.Store.UpdateExecutionStatus(r.Context(), execID, storage.ExecutionStatusCancelled, exec.State, &msg); err != nil {

		fmt.Printf("Warning: failed to update execution status: %v\n", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestartExecution handles POST /api/executions/{id}/restart

func (h *LifecycleHandler) RestartExecution(w http.ResponseWriter, r *http.Request) {
	execID := r.PathValue("id")
	if execID == "" {
		http.Error(w, "Missing execution ID", http.StatusBadRequest)
		return
	}

	// Get the original execution
	exec, err := h.Store.GetExecution(r.Context(), execID)
	if err != nil {
		http.Error(w, "Execution not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Check if execution can be restarted (must not be running)
	if exec.Status == storage.ExecutionStatusRunning {
		http.Error(w, "Cannot restart a running execution. Stop it first.", http.StatusBadRequest)
		return
	}

	// Get the workflow definition
	workflow, err := h.Store.GetWorkflow(r.Context(), exec.WorkflowID)
	if err != nil {
		http.Error(w, "Workflow not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Parse workflow definition
	var wf engine.Workflow
	if err := json.Unmarshal(workflow.Definition, &wf); err != nil {
		http.Error(w, "Failed to parse workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create new execution context
	ctx := engine.NewExecutionContext(wf.ID)
	runner := engine.NewWorkflowRunner(ctx, h.WorkerPool, h.Store, h.Registry, h.BlockRegistry)

	go func() {
		execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := runner.Run(execCtx, wf); err != nil {
			fmt.Printf("Restarted workflow execution failed: %v\n", err)
		}
	}()

	// Return the new execution ID (created by runner)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":               "Workflow restarted successfully",
		"original_execution_id": execID,
		"status":                "running",
	})
}

// BatchStopExecutions handles POST /api/executions/batch/stop

func (h *LifecycleHandler) BatchStopExecutions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExecutionIDs []string `json:"execution_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.ExecutionIDs) == 0 {
		http.Error(w, "No execution IDs provided", http.StatusBadRequest)
		return
	}

	results := make(map[string]string)
	for _, execID := range req.ExecutionIDs {
		if err := h.Registry.Cancel(execID); err != nil {
			results[execID] = "failed: " + err.Error()
		} else {
			results[execID] = "stopped"

			// Update status in database
			msg := "Execution stopped by batch operation"
			exec, _ := h.Store.GetExecution(r.Context(), execID)
			if exec != nil {
				h.Store.UpdateExecutionStatus(r.Context(), execID, storage.ExecutionStatusCancelled, exec.State, &msg)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}
