package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zarazaex69/conv3n/internal/api"
	"github.com/zarazaex69/conv3n/internal/storage"
)

func newExecutionMux(t *testing.T) (*http.ServeMux, storage.Storage) {
	store := newTestStorage(t)
	handler := api.NewExecutionHandler(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/workflows/{id}/executions", handler.ListByWorkflow)
	mux.HandleFunc("GET /api/executions/{id}", handler.Get)
	mux.HandleFunc("GET /api/executions/{id}/nodes/{nodeId}", handler.GetNodeResult)

	return mux, store
}

func TestExecutionAPI_ListByWorkflow(t *testing.T) {
	mux, store := newExecutionMux(t)
	ctx := testCtx

	// Create workflow
	wfID := "wf-1"

	// Create executions
	_, err := store.CreateExecution(ctx, wfID)
	if err != nil {
		t.Fatalf("failed to create execution: %v", err)
	}
	_, err = store.CreateExecution(ctx, wfID)
	if err != nil {
		t.Fatalf("failed to create execution: %v", err)
	}

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/workflows/"+wfID+"/executions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var execs []api.ExecutionResponse
	if err := json.NewDecoder(rec.Body).Decode(&execs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(execs) != 2 {
		t.Errorf("expected 2 executions, got %d", len(execs))
	}
}

func TestExecutionAPI_Get(t *testing.T) {
	mux, store := newExecutionMux(t)
	ctx := testCtx

	// Create execution
	execID, err := store.CreateExecution(ctx, "wf-1")
	if err != nil {
		t.Fatalf("failed to create execution: %v", err)
	}

	// Get
	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+execID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp api.ExecutionDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != execID {
		t.Errorf("expected ID %s, got %s", execID, resp.ID)
	}
}

func TestExecutionAPI_GetNodeResult(t *testing.T) {
	mux, store := newExecutionMux(t)
	ctx := testCtx

	// Create execution
	execID, err := store.CreateExecution(ctx, "wf-1")
	if err != nil {
		t.Fatalf("failed to create execution: %v", err)
	}

	// Save node result
	nodeID := "node-1"
	result := []byte(`{"foo":"bar"}`)
	store.SaveNodeResult(ctx, execID, nodeID, result)

	// Get node result
	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+execID+"/nodes/"+nodeID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != string(result) {
		t.Errorf("expected body %s, got %s", result, rec.Body.String())
	}
}

func TestExecutionAPI_NotFound(t *testing.T) {
	mux, _ := newExecutionMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/executions/non-existent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}
