package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type testHelper struct {
	t             *testing.T
	store         storage.Storage
	blockRegistry *engine.BlockRegistry
	workerPool    *engine.WorkerPool
	execRegistry  *engine.ExecutionRegistry
}

func setupTest(t *testing.T) *testHelper {
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	blockRegistry := engine.NewBlockRegistry("../../pkg/blocks")
	if err := blockRegistry.LoadFromDirectory("../../pkg/blocks"); err != nil {
		t.Fatalf("Failed to load blocks: %v", err)
	}

	workerPool, err := engine.NewWorkerPool(2, "bun", "../../pkg/bunock/worker_server.ts")
	if err != nil {
		store.Close()
		t.Fatalf("Failed to create worker pool: %v", err)
	}

	execRegistry := engine.NewExecutionRegistry()

	return &testHelper{
		t:             t,
		store:         store,
		blockRegistry: blockRegistry,
		workerPool:    workerPool,
		execRegistry:  execRegistry,
	}
}

func (h *testHelper) cleanup() {
	h.workerPool.Shutdown()
	h.store.Close()
}

func (h *testHelper) runWorkflow(workflow engine.Workflow) error {
	execCtx := engine.NewExecutionContext(workflow.ID)
	execCtx.ExecutionID = "test-" + workflow.ID

	runner := engine.NewWorkflowRunner(
		execCtx,
		h.workerPool,
		h.store,
		h.execRegistry,
		h.blockRegistry,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return runner.Run(ctx, workflow)
}

func TestE2E_HTTPRequestBlock(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-http-request",
		Name: "Test HTTP Request",
		Nodes: map[string]engine.Node{
			"http": {
				ID:   "http",
				Type: "std/http_request",
				Config: map[string]interface{}{
					"url":    "https://jsonplaceholder.typicode.com/users/1",
					"method": "GET",
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("HTTP request workflow failed: %v", err)
	}

	t.Log("HTTP request block test passed")
}

func TestE2E_TransformBlock(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-transform",
		Name: "Test Transform",
		Nodes: map[string]engine.Node{
			"transform": {
				ID:   "transform",
				Type: "std/transform",
				Config: map[string]interface{}{
					"input": map[string]interface{}{
						"name":  "John",
						"age":   30,
						"email": "john@example.com",
					},
					"operations": []map[string]interface{}{
						{
							"type":   "pick",
							"fields": []string{"name", "email"},
						},
					},
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("Transform workflow failed: %v", err)
	}

	t.Log("Transform block test passed")
}

func TestE2E_ConditionBlock(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-condition",
		Name: "Test Condition",
		Nodes: map[string]engine.Node{
			"condition": {
				ID:   "condition",
				Type: "std/condition",
				Config: map[string]interface{}{
					"expression": "input.value > 10",
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("Condition workflow failed: %v", err)
	}

	t.Log("Condition block test passed")
}

func TestE2E_DelayBlock(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-delay",
		Name: "Test Delay",
		Nodes: map[string]engine.Node{
			"delay": {
				ID:   "delay",
				Type: "std/delay",
				Config: map[string]interface{}{
					"duration": 100,
				},
			},
		},
		Edges: []engine.Edge{},
	}

	start := time.Now()
	err := h.runWorkflow(workflow)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Delay workflow failed: %v", err)
	}

	if duration < 100*time.Millisecond {
		t.Errorf("Delay was too short: %v", duration)
	}

	t.Log("Delay block test passed")
}

func TestE2E_FileBlock_Write(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	workflow := engine.Workflow{
		ID:   "test-file-write",
		Name: "Test File Write",
		Nodes: map[string]engine.Node{
			"write": {
				ID:   "write",
				Type: "std/file",
				Config: map[string]interface{}{
					"path": testFile,
					"operation": map[string]interface{}{
						"type":    "write",
						"content": "Hello from Conv3n!",
					},
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("File write workflow failed: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	if string(content) != "Hello from Conv3n!" {
		t.Errorf("File content mismatch: got %q", string(content))
	}

	t.Log("File write block test passed")
}

func TestE2E_FileBlock_Read(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(testFile, []byte("Test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	workflow := engine.Workflow{
		ID:   "test-file-read",
		Name: "Test File Read",
		Nodes: map[string]engine.Node{
			"read": {
				ID:   "read",
				Type: "std/file",
				Config: map[string]interface{}{
					"path": testFile,
					"operation": map[string]interface{}{
						"type": "read",
					},
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err = h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("File read workflow failed: %v", err)
	}

	t.Log("File read block test passed")
}

func TestE2E_CounterBlock(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-counter",
		Name: "Test Counter",
		Nodes: map[string]engine.Node{
			"counter": {
				ID:   "counter",
				Type: "std/counter",
				Config: map[string]interface{}{
					"counterName": "test_counter",
					"increment":   5,
					"scope":       "execution",
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("Counter workflow failed: %v", err)
	}

	t.Log("Counter block test passed")
}

func TestE2E_WebhookBlock(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-webhook",
		Name: "Test Webhook",
		Nodes: map[string]engine.Node{
			"webhook": {
				ID:   "webhook",
				Type: "std/webhook",
				Config: map[string]interface{}{
					"url":    "https://httpbin.org/post",
					"method": "POST",
					"body": map[string]interface{}{
						"message": "test",
					},
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("Webhook workflow failed: %v", err)
	}

	t.Log("Webhook block test passed")
}

func TestE2E_LoopBlock(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-loop",
		Name: "Test Loop",
		Nodes: map[string]engine.Node{
			"loop": {
				ID:   "loop",
				Type: "std/loop",
				Config: map[string]interface{}{
					"items": []interface{}{1, 2, 3, 4, 5},
				},
			},
		},
		Edges: []engine.Edge{},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("Loop workflow failed: %v", err)
	}

	t.Log("Loop block test passed")
}

func TestE2E_ChainedBlocks(t *testing.T) {
	h := setupTest(t)
	defer h.cleanup()

	workflow := engine.Workflow{
		ID:   "test-chain",
		Name: "Test Chained Blocks",
		Nodes: map[string]engine.Node{
			"http": {
				ID:   "http",
				Type: "std/http_request",
				Config: map[string]interface{}{
					"url":    "https://jsonplaceholder.typicode.com/users/1",
					"method": "GET",
				},
			},
			"transform": {
				ID:   "transform",
				Type: "std/transform",
				Config: map[string]interface{}{
					"operations": []map[string]interface{}{
						{
							"type":   "pick",
							"fields": []string{"id", "name", "email"},
						},
					},
				},
			},
			"condition": {
				ID:   "condition",
				Type: "std/condition",
				Config: map[string]interface{}{
					"expression": "input.id === 1",
				},
			},
		},
		Edges: []engine.Edge{
			{
				ID:     "e1",
				Source: "http",
				Target: "transform",
			},
			{
				ID:     "e2",
				Source: "transform",
				Target: "condition",
			},
		},
	}

	err := h.runWorkflow(workflow)
	if err != nil {
		t.Fatalf("Chained blocks workflow failed: %v", err)
	}

	t.Log("Chained blocks test passed")
}
