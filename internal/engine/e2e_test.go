package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

func TestE2ECompleteWorkflow(t *testing.T) {
	t.Log("Step 1: Creating in-memory storage...")
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	t.Log("Step 2: Loading block registry...")
	blockRegistry := engine.NewBlockRegistry("../../pkg/blocks")
	if err := blockRegistry.LoadFromDirectory("../../pkg/blocks"); err != nil {
		t.Fatalf("Failed to load blocks: %v", err)
	}

	t.Log("Step 3: Creating worker pool with 2 workers...")
	workerPool, err := engine.NewWorkerPool(
		2,
		"bun",
		"../../pkg/bunock/worker_server.ts",
	)
	if err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}
	defer workerPool.Shutdown()

	t.Log("Step 4: Creating execution registry...")
	execRegistry := engine.NewExecutionRegistry()

	t.Log("Step 5: Building complete workflow...")
	workflow := createCompleteWorkflow()

	t.Log("Step 6: Creating execution context...")
	execCtx := engine.NewExecutionContext(workflow.ID)
	execCtx.ExecutionID = "e2e-test-exec-001"

	t.Log("Step 7: Creating workflow runner...")
	runner := engine.NewWorkflowRunner(
		execCtx,
		workerPool,
		store,
		execRegistry,
		blockRegistry,
	)

	t.Log("Step 8: Executing workflow...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = runner.Run(ctx, workflow)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	t.Log("Step 9: Verifying execution results...")
	exec, err := store.GetExecution(ctx, execCtx.ExecutionID)
	if err != nil {
		t.Fatalf("Failed to get execution from storage: %v", err)
	}

	t.Logf("Execution completed with status: %s", exec.Status)
	t.Logf("Execution started at: %v", exec.StartedAt)
	t.Logf("Execution completed at: %v", exec.CompletedAt)

	if exec.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", exec.Status)
		if exec.Error != nil && *exec.Error != "" {
			t.Errorf("Execution error: %s", *exec.Error)
		}
	}

	t.Log("Step 10: Checking individual node results...")

	httpResult, err := store.GetNodeResult(ctx, execCtx.ExecutionID, "fetch_user")
	if err != nil {
		t.Logf("Warning: Could not get HTTP node result: %v", err)
	} else {
		t.Logf("HTTP request result: %+v", httpResult)
	}

	transformResult, err := store.GetNodeResult(ctx, execCtx.ExecutionID, "transform_data")
	if err != nil {
		t.Logf("Warning: Could not get transform node result: %v", err)
	} else {
		t.Logf("Transform result: %+v", transformResult)
	}

	t.Log("E2E test completed successfully!")
}

func createCompleteWorkflow() engine.Workflow {
	return engine.Workflow{
		ID:   "e2e-complete-test",
		Name: "E2E Complete Workflow Test",
		Nodes: map[string]engine.Node{
			"fetch_user": {
				ID:       "fetch_user",
				Type:     "std/http_request",
				Position: engine.Position{X: 100, Y: 100},
				Config: map[string]interface{}{
					"url":    "https://jsonplaceholder.typicode.com/users/1",
					"method": "GET",
				},
			},
			"transform_data": {
				ID:       "transform_data",
				Type:     "std/transform",
				Position: engine.Position{X: 350, Y: 100},
				Config: map[string]interface{}{
					"operations": []map[string]interface{}{
						{
							"type": "map",
							"expression": `(data) => ({
								id: data.id,
								username: data.username,
								email: data.email,
								city: data.address?.city || "Unknown",
								company: data.company?.name || "Unknown",
								processed_at: new Date().toISOString(),
								success: true
							})`,
						},
					},
				},
			},
			"check_success": {
				ID:       "check_success",
				Type:     "std/condition",
				Position: engine.Position{X: 600, Y: 100},
				Config: map[string]interface{}{
					"expression": "input.success === true",
				},
			},
			"final_transform": {
				ID:       "final_transform",
				Type:     "std/transform",
				Position: engine.Position{X: 850, Y: 100},
				Config: map[string]interface{}{
					"operations": []map[string]interface{}{
						{
							"type": "map",
							"expression": `(data) => ({
								message: "User data processed successfully",
								user_email: data.email,
								timestamp: new Date().toISOString()
							})`,
						},
					},
				},
			},
		},
		Edges: []engine.Edge{
			{
				ID:           "e1",
				Source:       "fetch_user",
				Target:       "transform_data",
				SourceHandle: "success",
				TargetHandle: "main",
			},
			{
				ID:           "e2",
				Source:       "transform_data",
				Target:       "check_success",
				SourceHandle: "default",
				TargetHandle: "main",
			},
			{
				ID:           "e3",
				Source:       "check_success",
				Target:       "final_transform",
				SourceHandle: "true",
				TargetHandle: "main",
			},
		},
		Config: &engine.WorkflowConfig{
			MaxConcurrentNodes: 2,
		},
	}
}
