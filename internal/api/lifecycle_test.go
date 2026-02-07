package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type mockStorage struct {
	storage.Storage
	executions map[string]*storage.Execution
	workflows  map[string]*storage.Workflow
	mu         sync.RWMutex
}

func (m *mockStorage) CreateExecution(ctx context.Context, workflowID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.executions == nil {
		m.executions = make(map[string]*storage.Execution)
	}

	execID := fmt.Sprintf("exec-%s-%d", workflowID, time.Now().UnixNano())
	m.executions[execID] = &storage.Execution{
		ID:         execID,
		WorkflowID: workflowID,
		Status:     storage.ExecutionStatusRunning,
		StartedAt:  time.Now(),
	}

	return execID, nil
}

func (m *mockStorage) GetExecution(ctx context.Context, id string) (*storage.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if exec, ok := m.executions[id]; ok {
		return exec, nil
	}
	return nil, fmt.Errorf("execution not found")
}

func (m *mockStorage) UpdateExecutionStatus(ctx context.Context, id string, status storage.ExecutionStatus, state []byte, errMsg *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exec, ok := m.executions[id]; ok {
		exec.Status = status
		exec.State = state
		exec.Error = errMsg
		return nil
	}
	return fmt.Errorf("execution not found")
}

func (m *mockStorage) GetWorkflow(ctx context.Context, id string) (*storage.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if wf, ok := m.workflows[id]; ok {
		return wf, nil
	}
	return nil, fmt.Errorf("workflow not found")
}

type mockRegistry struct {
	cancelled   map[string]bool
	cancelCalls int
	mu          sync.RWMutex
}

func newMockRegistry() *engine.ExecutionRegistry {
	return engine.NewExecutionRegistry()
}

func TestStopExecution_MissingID(t *testing.T) {
	handler := &LifecycleHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/executions//stop", nil)
	w := httptest.NewRecorder()

	handler.StopExecution(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func stringReader(s string) *bytes.Reader {
	return bytes.NewReader([]byte(s))
}

func countGoroutines() int {
	return runtime.NumGoroutine()
}

func TestStopExecution_NotFound(t *testing.T) {
	store := &mockStorage{
		executions: make(map[string]*storage.Execution),
	}

	handler := NewLifecycleHandler(store, newMockRegistry(), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/executions/nonexistent/stop", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handler.StopExecution(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestStopExecution_NotRunning(t *testing.T) {
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {
				ID:         "exec-1",
				WorkflowID: "wf-1",
				Status:     storage.ExecutionStatusCompleted,
				State:      []byte("{}"),
			},
		},
	}

	handler := NewLifecycleHandler(store, newMockRegistry(), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/executions/exec-1/stop", nil)
	req.SetPathValue("id", "exec-1")
	w := httptest.NewRecorder()

	handler.StopExecution(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !contains(w.Body.String(), "not running") {
		t.Errorf("expected error message about status, got: %s", w.Body.String())
	}
}

func TestStopExecution_Success(t *testing.T) {
	registry := newMockRegistry()
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {
				ID:         "exec-1",
				WorkflowID: "wf-1",
				Status:     storage.ExecutionStatusRunning,
				State:      []byte("{}"),
			},
		},
	}

	handler := NewLifecycleHandler(store, registry, nil, nil)

	_, cancel := context.WithCancel(context.Background())
	registry.Register("exec-1", cancel)

	req := httptest.NewRequest(http.MethodPost, "/api/executions/exec-1/stop", nil)
	req.SetPathValue("id", "exec-1")
	w := httptest.NewRecorder()

	handler.StopExecution(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	if registry.IsActive("exec-1") {
		t.Error("execution should not be active after stop")
	}

	store.mu.RLock()
	if store.executions["exec-1"].Status != storage.ExecutionStatusCancelled {
		t.Errorf("expected status cancelled, got %v", store.executions["exec-1"].Status)
	}
	store.mu.RUnlock()
}

func TestStopExecution_ConcurrentStops(t *testing.T) {
	registry := newMockRegistry()
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {
				ID:         "exec-1",
				WorkflowID: "wf-1",
				Status:     storage.ExecutionStatusRunning,
				State:      []byte("{}"),
			},
		},
	}

	handler := NewLifecycleHandler(store, registry, nil, nil)

	_, cancel := context.WithCancel(context.Background())
	registry.Register("exec-1", cancel)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/executions/exec-1/stop", nil)
			req.SetPathValue("id", "exec-1")
			w := httptest.NewRecorder()

			handler.StopExecution(w, req)

			mu.Lock()
			if w.Code == http.StatusNoContent {
				successCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	if successCount < 1 {
		t.Errorf("expected at least 1 successful stop, got %d", successCount)
	}
}

func TestRestartExecution_CannotRestartRunning(t *testing.T) {
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {
				ID:         "exec-1",
				WorkflowID: "wf-1",
				Status:     storage.ExecutionStatusRunning,
			},
		},
	}

	handler := NewLifecycleHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/executions/exec-1/restart", nil)
	req.SetPathValue("id", "exec-1")
	w := httptest.NewRecorder()

	handler.RestartExecution(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !contains(w.Body.String(), "Cannot restart a running execution") {
		t.Errorf("expected specific error message, got: %s", w.Body.String())
	}
}

func TestRestartExecution_WorkflowNotFound(t *testing.T) {
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {
				ID:         "exec-1",
				WorkflowID: "wf-missing",
				Status:     storage.ExecutionStatusFailed,
			},
		},
		workflows: make(map[string]*storage.Workflow),
	}

	handler := NewLifecycleHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/executions/exec-1/restart", nil)
	req.SetPathValue("id", "exec-1")
	w := httptest.NewRecorder()

	handler.RestartExecution(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestRestartExecution_InvalidWorkflowDefinition(t *testing.T) {
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {
				ID:         "exec-1",
				WorkflowID: "wf-1",
				Status:     storage.ExecutionStatusFailed,
			},
		},
		workflows: map[string]*storage.Workflow{
			"wf-1": {
				ID:         "wf-1",
				Definition: []byte("invalid json{{{"),
			},
		},
	}

	handler := NewLifecycleHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/executions/exec-1/restart", nil)
	req.SetPathValue("id", "exec-1")
	w := httptest.NewRecorder()

	handler.RestartExecution(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestBatchStopExecutions_EmptyArray(t *testing.T) {
	handler := NewLifecycleHandler(nil, newMockRegistry(), nil, nil)

	body := `{"execution_ids": []}`
	req := httptest.NewRequest(http.MethodPost, "/api/executions/batch/stop", stringReader(body))
	w := httptest.NewRecorder()

	handler.BatchStopExecutions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBatchStopExecutions_InvalidJSON(t *testing.T) {
	handler := NewLifecycleHandler(nil, newMockRegistry(), nil, nil)

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/executions/batch/stop", stringReader(body))
	w := httptest.NewRecorder()

	handler.BatchStopExecutions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBatchStopExecutions_PartialSuccess(t *testing.T) {
	registry := newMockRegistry()
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {ID: "exec-1", Status: storage.ExecutionStatusRunning, State: []byte("{}")},
			"exec-2": {ID: "exec-2", Status: storage.ExecutionStatusRunning, State: []byte("{}")},
		},
	}

	handler := NewLifecycleHandler(store, registry, nil, nil)

	// Register executions in registry to make them stoppable
	for _, id := range []string{"exec-1", "exec-2"} {
		_, cancel := context.WithCancel(context.Background())
		registry.Register(id, cancel)
	}

	body := `{"execution_ids": ["exec-1", "exec-2", "exec-nonexistent"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/executions/batch/stop", stringReader(body))
	w := httptest.NewRecorder()

	handler.BatchStopExecutions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	results, ok := result["results"].(map[string]interface{})
	if !ok {
		t.Fatal("expected results map")
	}

	if results["exec-1"] != "stopped" {
		t.Errorf("exec-1 should be stopped, got: %v", results["exec-1"])
	}
	if results["exec-2"] != "stopped" {
		t.Errorf("exec-2 should be stopped, got: %v", results["exec-2"])
	}
	if !contains(results["exec-nonexistent"].(string), "failed") {
		t.Errorf("exec-nonexistent should fail, got: %v", results["exec-nonexistent"])
	}
}

func TestBatchStopExecutions_Concurrent(t *testing.T) {
	registry := newMockRegistry()
	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {ID: "exec-1", Status: storage.ExecutionStatusRunning, State: []byte("{}")},
			"exec-2": {ID: "exec-2", Status: storage.ExecutionStatusRunning, State: []byte("{}")},
			"exec-3": {ID: "exec-3", Status: storage.ExecutionStatusRunning, State: []byte("{}")},
		},
	}

	handler := NewLifecycleHandler(store, registry, nil, nil)

	for _, id := range []string{"exec-1", "exec-2", "exec-3"} {
		_, cancel := context.WithCancel(context.Background())
		registry.Register(id, cancel)
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"execution_ids": ["exec-1", "exec-2", "exec-3"]}`
			req := httptest.NewRequest(http.MethodPost, "/api/executions/batch/stop", stringReader(body))
			w := httptest.NewRecorder()

			handler.BatchStopExecutions(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
		}()
	}

	wg.Wait()

	if registry.ActiveCount() > 0 {
		t.Errorf("expected 0 active executions, got %d", registry.ActiveCount())
	}
}

func TestRestartExecution_NoGoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping goroutine leak test in short mode")
	}

	wfDef := engine.Workflow{
		ID:   "wf-1",
		Name: "test",
		Nodes: map[string]engine.Node{
			"start": {ID: "start", Type: "delay"},
		},
	}
	defBytes, _ := json.Marshal(wfDef)

	store := &mockStorage{
		executions: map[string]*storage.Execution{
			"exec-1": {ID: "exec-1", WorkflowID: "wf-1", Status: storage.ExecutionStatusFailed},
		},
		workflows: map[string]*storage.Workflow{
			"wf-1": {ID: "wf-1", Definition: defBytes},
		},
	}

	handler := NewLifecycleHandler(store, newMockRegistry(), nil, nil)

	before := countGoroutines()

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/executions/exec-1/restart", nil)
		req.SetPathValue("id", "exec-1")
		w := httptest.NewRecorder()

		handler.RestartExecution(w, req)
	}

	time.Sleep(100 * time.Millisecond)

	after := countGoroutines()

	if after > before+15 {
		t.Errorf("potential goroutine leak: before=%d after=%d", before, after)
	}
}
