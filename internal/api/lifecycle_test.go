package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zarazaex69/conv3n/internal/api"
	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

func newLifecycleMux(t *testing.T) (*http.ServeMux, storage.Storage, *engine.ExecutionRegistry) {
	store := newTestStorage(t)
	registry := engine.NewExecutionRegistry()

	handler := api.NewLifecycleHandler(store, registry, t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/executions/{id}/stop", handler.StopExecution)
	mux.HandleFunc("POST /api/executions/{id}/restart", handler.RestartExecution)
	mux.HandleFunc("POST /api/executions/batch/stop", handler.BatchStopExecutions)

	return mux, store, registry
}

func TestLifecycleAPI_StopExecution(t *testing.T) {
	mux, store, registry := newLifecycleMux(t)
	ctx := testCtx

	// Create running execution
	execID, _ := store.CreateExecution(ctx, "wf-1")

	// Register it as active
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry.Register(execID, cancel)

	// Stop it
	req := httptest.NewRequest(http.MethodPost, "/api/executions/"+execID+"/stop", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it's cancelled in registry
	if registry.IsActive(execID) {
		t.Error("execution should be removed from registry")
	}

	// Verify status in DB
	exec, _ := store.GetExecution(ctx, execID)
	if exec.Status != storage.ExecutionStatusCancelled {
		t.Errorf("expected status cancelled, got %s", exec.Status)
	}
}

func TestLifecycleAPI_RestartExecution(t *testing.T) {
	mux, store, _ := newLifecycleMux(t)
	ctx := testCtx

	// Create workflow
	wfDef := map[string]interface{}{
		"id":    "wf-1",
		"nodes": map[string]interface{}{},
		"edges": []interface{}{},
	}
	wfBytes, _ := json.Marshal(wfDef)
	store.CreateWorkflow(ctx, &storage.Workflow{ID: "wf-1", Name: "Test Workflow", Definition: wfBytes})

	// Create completed execution
	execID, _ := store.CreateExecution(ctx, "wf-1")
	err := store.UpdateExecutionStatus(ctx, execID, storage.ExecutionStatusCompleted, []byte("{}"), nil)
	if err != nil {
		t.Fatalf("failed to update execution status: %v", err)
	}

	// Restart
	req := httptest.NewRequest(http.MethodPost, "/api/executions/"+execID+"/restart", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLifecycleAPI_BatchStop(t *testing.T) {
	mux, store, registry := newLifecycleMux(t)
	ctx := testCtx

	// Create 2 running executions
	id1, _ := store.CreateExecution(ctx, "wf-1")
	id2, _ := store.CreateExecution(ctx, "wf-1")

	registry.Register(id1, func() {})
	registry.Register(id2, func() {})

	// Batch stop
	body := map[string]interface{}{
		"execution_ids": []string{id1, id2},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/executions/batch/stop", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify both stopped
	if registry.IsActive(id1) || registry.IsActive(id2) {
		t.Error("executions should be stopped")
	}
}
