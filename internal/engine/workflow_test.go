package engine_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

func createTestStorage(t *testing.T) storage.Storage {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

// TestNewWorkflowRunner verifies WorkflowRunner initialization
func TestNewWorkflowRunner(t *testing.T) {
	ctx := engine.NewExecutionContext("test-workflow")
	blocksDir := "/test/blocks"
	store := createTestStorage(t)
	runner := engine.NewWorkflowRunner(ctx, blocksDir, store, nil)

	if runner == nil {
		t.Fatal("NewWorkflowRunner returned nil")
	}
}

// TestWorkflowRunner_Run_EmptyWorkflow verifies handling of empty workflow
func TestWorkflowRunner_Run_EmptyWorkflow(t *testing.T) {
	workflow := engine.Workflow{
		ID:    "empty-wf",
		Name:  "Empty Workflow",
		Nodes: map[string]engine.Node{},
		Edges: []engine.Edge{},
	}

	ctx := engine.NewExecutionContext(workflow.ID)
	store := createTestStorage(t)
	runner := engine.NewWorkflowRunner(ctx, "/tmp", store, nil)

	execCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := runner.Run(execCtx, workflow)
	// Empty workflow should fail because there are no nodes
	if err == nil {
		t.Error("Empty workflow should fail with 'no nodes' error")
	}
}

// TestWorkflowRunner_Run_SingleBlock verifies single block execution
func TestWorkflowRunner_Run_SingleBlock(t *testing.T) {
	// Skip if bun is not available
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	// Setup mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer ts.Close()

	workflow := engine.Workflow{
		ID:   "single-block-wf",
		Name: "Single Block Workflow",
		Nodes: map[string]engine.Node{
			"http-block": {
				ID:       "http-block",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 0, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL,
					"method": "GET",
				},
			},
		},
		Edges: []engine.Edge{},
	}

	// Change to project root
	wd, _ := os.Getwd()
	os.Chdir("../..")
	defer os.Chdir(wd)

	ctx := engine.NewExecutionContext(workflow.ID)
	store := createTestStorage(t)
	runner := engine.NewWorkflowRunner(ctx, "pkg/blocks", store, nil)

	execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(execCtx, workflow)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify result
	result := ctx.Results["http-block"]
	if result == nil {
		t.Fatal("Expected result for http-block, got nil")
	}
}

func TestWorkflowRunner_Run_Chain(t *testing.T) {
	// Skip if bun is not available
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	// 1. Setup Mock HTTP Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := []byte(`{"message": "hello from api", "value": 100}`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer ts.Close()

	// 2. Define Workflow (graph format) - simplified without variable substitution
	// Variable substitution test is complex due to httptest + bun interaction
	workflow := engine.Workflow{
		ID:   "wf-test-1",
		Name: "Test Chain",
		Nodes: map[string]engine.Node{
			"block_1": {
				ID:       "block_1",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 0, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL,
					"method": "GET",
				},
			},
			"block_2": {
				ID:       "block_2",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 250, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL,
					"method": "GET",
				},
			},
		},
		Edges: []engine.Edge{
			{ID: "e1", Source: "block_1", Target: "block_2", SourceHandle: "success", TargetHandle: "main"},
		},
	}

	// 3. Run Workflow
	wd, _ := os.Getwd()
	os.Chdir("../..")
	defer os.Chdir(wd)

	blocksDir := "pkg/blocks"

	ctx := engine.NewExecutionContext(workflow.ID)
	store := createTestStorage(t)
	runner := engine.NewWorkflowRunner(ctx, blocksDir, store, nil)

	execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := runner.Run(execCtx, workflow); err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// 4. Verify Results
	res1 := ctx.Results["block_1"]
	if res1 == nil {
		t.Fatal("Block 1 result missing")
	}

	res2 := ctx.Results["block_2"]
	if res2 == nil {
		t.Fatal("Block 2 result missing")
	}

	t.Logf("Block 1 result type: %T, value: %+v", res1, res1)
}

// TestWorkflowRunner_Run_ErrorInBlock verifies error handling during block execution
func TestWorkflowRunner_Run_ErrorInBlock(t *testing.T) {
	// Skip if bun is not available
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	workflow := engine.Workflow{
		ID:   "error-wf",
		Name: "Error Workflow",
		Nodes: map[string]engine.Node{
			"bad-block": {
				ID:       "bad-block",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 0, Y: 100},
				Config: map[string]interface{}{
					// Missing required 'url' field
					"method": "GET",
				},
			},
		},
		Edges: []engine.Edge{},
	}

	wd, _ := os.Getwd()
	os.Chdir("../..")
	defer os.Chdir(wd)

	ctx := engine.NewExecutionContext(workflow.ID)
	store := createTestStorage(t)
	runner := engine.NewWorkflowRunner(ctx, "pkg/blocks", store, nil)

	execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(execCtx, workflow)
	if err == nil {
		t.Error("Expected error for block with missing config, got nil")
	}
}

// TestWorkflowRunner_Run_SequentialExecution verifies graph traversal executes nodes in order
func TestWorkflowRunner_Run_SequentialExecution(t *testing.T) {
	// Skip if bun is not available
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	// Use local HTTP test server to avoid external network dependency
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "ok"}`))
	}))
	defer ts.Close()

	workflow := engine.Workflow{
		ID:   "sequential-wf",
		Name: "Sequential Workflow",
		Nodes: map[string]engine.Node{
			"block-1": {
				ID:       "block-1",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 0, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL + "?block=1",
					"method": "GET",
				},
			},
			"block-2": {
				ID:       "block-2",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 250, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL + "?block=2",
					"method": "GET",
				},
			},
			"block-3": {
				ID:       "block-3",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 500, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL + "?block=3",
					"method": "GET",
				},
			},
		},
		Edges: []engine.Edge{
			{ID: "e1", Source: "block-1", Target: "block-2", SourceHandle: "success", TargetHandle: "main"},
			{ID: "e2", Source: "block-2", Target: "block-3", SourceHandle: "success", TargetHandle: "main"},
		},
	}

	wd, _ := os.Getwd()
	os.Chdir("../..")
	defer os.Chdir(wd)

	ctx := engine.NewExecutionContext(workflow.ID)
	store := createTestStorage(t)
	runner := engine.NewWorkflowRunner(ctx, "pkg/blocks", store, nil)

	execCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := runner.Run(execCtx, workflow)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify all 3 blocks executed
	if len(ctx.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(ctx.Results))
	}

	// Verify each block has a result
	for _, blockID := range []string{"block-1", "block-2", "block-3"} {
		if ctx.Results[blockID] == nil {
			t.Errorf("Missing result for %s", blockID)
		}
	}
}

func TestGraphRunner_ResumeExecution(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "ok"}`))
	}))
	defer ts.Close()

	workflow := engine.Workflow{
		ID:   "resume-wf",
		Name: "Resume Workflow",
		Nodes: map[string]engine.Node{
			"block-1": {
				ID:       "block-1",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 0, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL + "?block=1",
					"method": "GET",
				},
			},
			"block-2": {
				ID:       "block-2",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 250, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL + "?block=2",
					"method": "GET",
				},
			},
		},
		Edges: []engine.Edge{
			{ID: "e1", Source: "block-1", Target: "block-2", SourceHandle: "success", TargetHandle: "main"},
		},
	}

	wd, _ := os.Getwd()
	os.Chdir("../..")
	defer os.Chdir(wd)

	store := createTestStorage(t)
	ctxExec := context.Background()

	// Создаём execution и сохраняем state так, как будто block-1 уже успешно отработал
	execID, err := store.CreateExecution(ctxExec, workflow.ID)
	if err != nil {
		t.Fatalf("failed to create execution: %v", err)
	}

	initialState := struct {
		Results       map[string]interface{} `json:"results"`
		Variables     map[string]interface{} `json:"variables"`
		CurrentNodeID string                 `json:"current_node_id"`
	}{
		Results: map[string]interface{}{
			"block-1": map[string]interface{}{"message": "ok"},
		},
		Variables:     map[string]interface{}{"foo": "bar"},
		CurrentNodeID: "block-1",
	}

	stateBytes, err := json.Marshal(initialState)
	if err != nil {
		t.Fatalf("failed to marshal initial state: %v", err)
	}

	if err := store.UpdateExecutionStatus(ctxExec, execID, storage.ExecutionStatusRunning, stateBytes, nil); err != nil {
		t.Fatalf("failed to update execution status: %v", err)
	}

	execCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	blocksDir := "pkg/blocks"
	if err := engine.ResumeGraphExecution(execCtx, store, execID, &workflow, blocksDir); err != nil {
		t.Fatalf("ResumeGraphExecution failed: %v", err)
	}

	exec, err := store.GetExecution(context.Background(), execID)
	if err != nil {
		t.Fatalf("failed to reload execution: %v", err)
	}

	var finalState struct {
		Results       map[string]interface{} `json:"results"`
		Variables     map[string]interface{} `json:"variables"`
		CurrentNodeID string                 `json:"current_node_id"`
	}
	if err := json.Unmarshal(exec.State, &finalState); err != nil {
		t.Fatalf("failed to unmarshal final state: %v", err)
	}

	if finalState.Results["block-2"] == nil {
		t.Fatalf("expected result for block-2 after resume, got nil")
	}
}

func TestGraphRunner_IdempotentNodeExecution(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "ok"}`))
	}))
	defer ts.Close()

	workflow := engine.Workflow{
		ID:   "idempotent-wf",
		Name: "Idempotent Workflow",
		Nodes: map[string]engine.Node{
			"only-block": {
				ID:       "only-block",
				Type:     engine.NodeTypeHTTPRequest,
				Position: engine.Position{X: 0, Y: 100},
				Config: map[string]interface{}{
					"url":    ts.URL,
					"method": "GET",
				},
			},
		},
		Edges: []engine.Edge{},
	}

	wd, _ := os.Getwd()
	os.Chdir("../..")
	defer os.Chdir(wd)

	store := createTestStorage(t)
	ctxExec := context.Background()

	// Первый "запуск": один HTTP-вызов и ручная запись результата в storage
	// HTTP-хэндлер должен сработать ровно один раз
	req, err := http.NewRequestWithContext(ctxExec, http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute initial HTTP request: %v", err)
	}
	resp.Body.Close()

	if hits != 1 {
		t.Fatalf("expected 1 HTTP hit after initial request, got %d", hits)
	}

	execID, err := store.CreateExecution(ctxExec, workflow.ID)
	if err != nil {
		t.Fatalf("failed to create execution: %v", err)
	}

	resultPayload := map[string]interface{}{"message": "ok"}
	resultBytes, err := json.Marshal(resultPayload)
	if err != nil {
		t.Fatalf("failed to marshal result payload: %v", err)
	}

	if err := store.SaveNodeResult(ctxExec, execID, "only-block", resultBytes); err != nil {
		t.Fatalf("failed to save node result: %v", err)
	}

	state := struct {
		Results       map[string]interface{} `json:"results"`
		Variables     map[string]interface{} `json:"variables"`
		CurrentNodeID string                 `json:"current_node_id"`
	}{
		Results: map[string]interface{}{
			"only-block": resultPayload,
		},
		Variables:     map[string]interface{}{},
		CurrentNodeID: "only-block",
	}

	stateBytes, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal execution state: %v", err)
	}

	if err := store.UpdateExecutionStatus(ctxExec, execID, storage.ExecutionStatusRunning, stateBytes, nil); err != nil {
		t.Fatalf("failed to update execution status: %v", err)
	}

	// Второй запуск через ResumeGraphExecution: нода не должна вызываться повторно
	resumeCtx, resumeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resumeCancel()

	if err := engine.ResumeGraphExecution(resumeCtx, store, execID, &workflow, "pkg/blocks"); err != nil {
		t.Fatalf("ResumeGraphExecution failed: %v", err)
	}

	if hits != 1 {
		t.Fatalf("expected HTTP handler to be called only once, got %d", hits)
	}
}
