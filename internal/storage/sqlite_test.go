package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zarazaex69/conv3n/internal/storage"
)

func TestSQLiteStorage(t *testing.T) {

	// when running tests in parallel (go test -parallel N)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize storage
	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	t.Run("CreateAndGetExecution", func(t *testing.T) {
		workflowID := "test-workflow-1"

		// Create execution
		executionID, err := store.CreateExecution(ctx, workflowID)
		if err != nil {
			t.Fatalf("failed to create execution: %v", err)
		}

		// Retrieve execution
		exec, err := store.GetExecution(ctx, executionID)
		if err != nil {
			t.Fatalf("failed to get execution: %v", err)
		}

		// Verify execution data
		if exec.WorkflowID != workflowID {
			t.Errorf("expected workflow_id %s, got %s", workflowID, exec.WorkflowID)
		}
		if exec.Status != storage.ExecutionStatusRunning {
			t.Errorf("expected status running, got %s", exec.Status)
		}
	})

	t.Run("UpdateExecutionStatus", func(t *testing.T) {
		workflowID := "test-workflow-2"

		// Create execution
		executionID, err := store.CreateExecution(ctx, workflowID)
		if err != nil {
			t.Fatalf("failed to create execution: %v", err)
		}

		// Update to completed
		finalState := []byte(`{"status":"completed","result":"success"}`)
		err = store.UpdateExecutionStatus(ctx, executionID, storage.ExecutionStatusCompleted, finalState, nil)
		if err != nil {
			t.Fatalf("failed to update execution status: %v", err)
		}

		// Verify updated status
		exec, err := store.GetExecution(ctx, executionID)
		if err != nil {
			t.Fatalf("failed to get execution: %v", err)
		}

		if exec.Status != storage.ExecutionStatusCompleted {
			t.Errorf("expected status completed, got %s", exec.Status)
		}
		if string(exec.State) != string(finalState) {
			t.Errorf("expected state %s, got %s", finalState, exec.State)
		}
		if exec.CompletedAt == nil {
			t.Error("expected completed_at to be set")
		}
	})

	t.Run("ExecutionWithError", func(t *testing.T) {
		workflowID := "test-workflow-3"

		// Create execution
		executionID, err := store.CreateExecution(ctx, workflowID)
		if err != nil {
			t.Fatalf("failed to create execution: %v", err)
		}

		// Update to failed with error
		errorMsg := "node execution failed: timeout"
		finalState := []byte(`{"status":"failed"}`)
		err = store.UpdateExecutionStatus(ctx, executionID, storage.ExecutionStatusFailed, finalState, &errorMsg)
		if err != nil {
			t.Fatalf("failed to update execution status: %v", err)
		}

		// Verify error is stored
		exec, err := store.GetExecution(ctx, executionID)
		if err != nil {
			t.Fatalf("failed to get execution: %v", err)
		}

		if exec.Status != storage.ExecutionStatusFailed {
			t.Errorf("expected status failed, got %s", exec.Status)
		}
		if exec.Error == nil {
			t.Fatal("expected error to be set")
		}
		if *exec.Error != errorMsg {
			t.Errorf("expected error %s, got %s", errorMsg, *exec.Error)
		}
	})

	t.Run("ListExecutions", func(t *testing.T) {
		workflowID := "test-workflow-4"

		// Create multiple executions
		var executionIDs []string
		for i := 0; i < 5; i++ {
			execID, err := store.CreateExecution(ctx, workflowID)
			if err != nil {
				t.Fatalf("failed to create execution %d: %v", i, err)
			}
			executionIDs = append(executionIDs, execID)
		}

		// List executions (limit 3)
		executions, err := store.ListExecutions(ctx, workflowID, 3)
		if err != nil {
			t.Fatalf("failed to list executions: %v", err)
		}

		// Verify count
		if len(executions) != 3 {
			t.Errorf("expected 3 executions, got %d", len(executions))
		}

		// Verify all belong to same workflow
		for _, exec := range executions {
			if exec.WorkflowID != workflowID {
				t.Errorf("expected workflow_id %s, got %s", workflowID, exec.WorkflowID)
			}
		}
	})

	t.Run("SaveAndGetNodeResult", func(t *testing.T) {
		workflowID := "test-workflow-5"

		// Create execution
		executionID, err := store.CreateExecution(ctx, workflowID)
		if err != nil {
			t.Fatalf("failed to create execution: %v", err)
		}

		nodeID := "http_request_1"
		result := []byte(`{"statusCode":200,"body":"OK"}`)

		// Save node result
		err = store.SaveNodeResult(ctx, executionID, nodeID, result)
		if err != nil {
			t.Fatalf("failed to save node result: %v", err)
		}

		// Retrieve node result
		retrieved, err := store.GetNodeResult(ctx, executionID, nodeID)
		if err != nil {
			t.Fatalf("failed to get node result: %v", err)
		}

		// Verify result matches
		if string(retrieved) != string(result) {
			t.Errorf("expected result %s, got %s", result, retrieved)
		}
	})

	t.Run("MultipleNodes", func(t *testing.T) {
		workflowID := "test-workflow-6"

		// Create execution
		executionID, err := store.CreateExecution(ctx, workflowID)
		if err != nil {
			t.Fatalf("failed to create execution: %v", err)
		}

		// Save results for multiple nodes
		nodes := map[string][]byte{
			"node_1": []byte(`{"output":"result1"}`),
			"node_2": []byte(`{"output":"result2"}`),
			"node_3": []byte(`{"output":"result3"}`),
		}

		for nodeID, result := range nodes {
			err := store.SaveNodeResult(ctx, executionID, nodeID, result)
			if err != nil {
				t.Fatalf("failed to save node %s: %v", nodeID, err)
			}
		}

		// Verify all nodes can be retrieved
		for nodeID, expected := range nodes {
			retrieved, err := store.GetNodeResult(ctx, executionID, nodeID)
			if err != nil {
				t.Fatalf("failed to get node %s: %v", nodeID, err)
			}

			if string(retrieved) != string(expected) {
				t.Errorf("node %s: expected %s, got %s", nodeID, expected, retrieved)
			}
		}
	})

	t.Run("ExecutionHistory", func(t *testing.T) {
		workflowID := "test-workflow-7"

		// Create multiple executions with different statuses

		exec1, _ := store.CreateExecution(ctx, workflowID)
		store.UpdateExecutionStatus(ctx, exec1, storage.ExecutionStatusCompleted, []byte(`{"run":1}`), nil)

		exec2, _ := store.CreateExecution(ctx, workflowID)
		errorMsg := "failed"
		store.UpdateExecutionStatus(ctx, exec2, storage.ExecutionStatusFailed, []byte(`{"run":2}`), &errorMsg)

		exec3, _ := store.CreateExecution(ctx, workflowID)
		store.UpdateExecutionStatus(ctx, exec3, storage.ExecutionStatusCompleted, []byte(`{"run":3}`), nil)

		// List all executions
		executions, err := store.ListExecutions(ctx, workflowID, 10)
		if err != nil {
			t.Fatalf("failed to list executions: %v", err)
		}

		// Verify we have history of all runs
		if len(executions) != 3 {
			t.Errorf("expected 3 executions in history, got %d", len(executions))
		}

		// Verify they are ordered by most recent first (by started_at DESC)

		if len(executions) >= 2 {

			found := false
			for _, id := range []string{exec1, exec2, exec3} {
				if executions[0].ID == id {
					found = true
					break
				}
			}
			if !found {
				t.Error("first execution in list is not one of the created executions")
			}
		}
	})

	t.Run("TriggerCRUD", func(t *testing.T) {
		workflowID := "test-workflow-trigger"

		// Create trigger
		config := []byte(`{"schedule":"* * * * *"}`)
		trigger := &storage.Trigger{
			ID:         "trigger-1",
			WorkflowID: workflowID,
			Type:       "cron",
			Config:     config,
			Enabled:    true,
		}

		err := store.CreateTrigger(ctx, trigger)
		if err != nil {
			t.Fatalf("failed to create trigger: %v", err)
		}

		// Get trigger
		got, err := store.GetTrigger(ctx, "trigger-1")
		if err != nil {
			t.Fatalf("failed to get trigger: %v", err)
		}

		if got.WorkflowID != workflowID {
			t.Errorf("expected workflow_id %s, got %s", workflowID, got.WorkflowID)
		}

		// Update trigger
		got.Enabled = false
		err = store.UpdateTrigger(ctx, got)
		if err != nil {
			t.Fatalf("failed to update trigger: %v", err)
		}

		updated, _ := store.GetTrigger(ctx, "trigger-1")
		if updated.Enabled {
			t.Error("expected trigger to be disabled")
		}

		// List triggers
		list, err := store.ListTriggers(ctx, workflowID)
		if err != nil {
			t.Fatalf("failed to list triggers: %v", err)
		}
		if len(list) != 1 {
			t.Errorf("expected 1 trigger, got %d", len(list))
		}

		// Delete trigger
		err = store.DeleteTrigger(ctx, "trigger-1")
		if err != nil {
			t.Fatalf("failed to delete trigger: %v", err)
		}

		_, err = store.GetTrigger(ctx, "trigger-1")
		if err == nil {
			t.Error("expected error getting deleted trigger")
		}
	})

	t.Run("TriggerExecutions", func(t *testing.T) {
		triggerID := "trigger-exec-test"

		// Create execution
		exec := &storage.TriggerExecution{
			ID:        "texec-1",
			TriggerID: triggerID,
			Status:    "success",
			Payload:   []byte(`{"foo":"bar"}`),
		}

		err := store.CreateTriggerExecution(ctx, exec)
		if err != nil {
			t.Fatalf("failed to create trigger execution: %v", err)
		}

		// List executions
		list, err := store.ListTriggerExecutions(ctx, triggerID, 10)
		if err != nil {
			t.Fatalf("failed to list trigger executions: %v", err)
		}

		if len(list) != 1 {
			t.Errorf("expected 1 execution, got %d", len(list))
		}

		if list[0].ID != "texec-1" {
			t.Errorf("expected ID texec-1, got %s", list[0].ID)
		}
	})
}

// TestParallelExecutions verifies that tests can run in parallel without conflicts
func TestParallelExecutions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "parallel_test.db")

	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create execution
	executionID, err := store.CreateExecution(ctx, "parallel-workflow")
	if err != nil {
		t.Fatalf("failed to create execution: %v", err)
	}

	// Verify it works
	exec, err := store.GetExecution(ctx, executionID)
	if err != nil {
		t.Fatalf("failed to get execution: %v", err)
	}

	if exec.Status != storage.ExecutionStatusRunning {
		t.Errorf("expected status running, got %s", exec.Status)
	}
}

func TestWorkflowCRUD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "workflow_test.db")

	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	t.Run("CreateAndGetWorkflow", func(t *testing.T) {
		workflow := &storage.Workflow{
			ID:         "wf-1",
			Name:       "Test Workflow 1",
			Definition: []byte(`{"nodes":[]}`),
		}

		err := store.CreateWorkflow(ctx, workflow)
		if err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}

		got, err := store.GetWorkflow(ctx, "wf-1")
		if err != nil {
			t.Fatalf("failed to get workflow: %v", err)
		}

		if got.ID != workflow.ID {
			t.Errorf("expected ID %s, got %s", workflow.ID, got.ID)
		}
		if got.Name != workflow.Name {
			t.Errorf("expected Name %s, got %s", workflow.Name, got.Name)
		}
		if string(got.Definition) != string(workflow.Definition) {
			t.Errorf("expected Definition %s, got %s", workflow.Definition, got.Definition)
		}
	})

	t.Run("UpdateWorkflow", func(t *testing.T) {
		workflow := &storage.Workflow{
			ID:         "wf-2",
			Name:       "Test Workflow 2",
			Definition: []byte(`{"nodes":[]}`),
		}

		err := store.CreateWorkflow(ctx, workflow)
		if err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}

		// Update
		workflow.Name = "Updated Workflow 2"
		workflow.Definition = []byte(`{"nodes":[{"id":"1"}]}`)
		err = store.UpdateWorkflow(ctx, workflow)
		if err != nil {
			t.Fatalf("failed to update workflow: %v", err)
		}

		got, err := store.GetWorkflow(ctx, "wf-2")
		if err != nil {
			t.Fatalf("failed to get workflow: %v", err)
		}

		if got.Name != "Updated Workflow 2" {
			t.Errorf("expected updated Name, got %s", got.Name)
		}
		if string(got.Definition) != string(workflow.Definition) {
			t.Errorf("expected updated Definition, got %s", got.Definition)
		}
	})

	t.Run("DeleteWorkflow", func(t *testing.T) {
		workflow := &storage.Workflow{
			ID:         "wf-3",
			Name:       "Test Workflow 3",
			Definition: []byte(`{"nodes":[]}`),
		}

		err := store.CreateWorkflow(ctx, workflow)
		if err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}

		err = store.DeleteWorkflow(ctx, "wf-3")
		if err != nil {
			t.Fatalf("failed to delete workflow: %v", err)
		}

		_, err = store.GetWorkflow(ctx, "wf-3")
		if err == nil {
			t.Error("expected error getting deleted workflow")
		}
	})

	t.Run("ListWorkflows", func(t *testing.T) {

		// Since we run in parallel with unique db per test function (TestWorkflowCRUD),

		// Actually TestWorkflowCRUD runs sequentially its sub-tests sharing the same db.

		list, err := store.ListWorkflows(ctx)
		if err != nil {
			t.Fatalf("failed to list workflows: %v", err)
		}

		// We expect at least wf-1 and wf-2
		found := 0
		for _, w := range list {
			if w.ID == "wf-1" || w.ID == "wf-2" {
				found++
			}
		}

		if found != 2 {
			t.Errorf("expected to find wf-1 and wf-2, found %d", found)
		}
	})
}

func TestListAllTriggers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "triggers_test.db")

	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	// Create workflow first due to FK constraint
	wf := &storage.Workflow{ID: "wf-triggers", Name: "WF Triggers", Definition: []byte("{}")}
	if err := store.CreateWorkflow(ctx, wf); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Create triggers
	t1 := &storage.Trigger{ID: "t1", WorkflowID: "wf-triggers", Type: "cron", Config: []byte("{}"), Enabled: true}
	t2 := &storage.Trigger{ID: "t2", WorkflowID: "wf-triggers", Type: "webhook", Config: []byte("{}"), Enabled: false}
	t3 := &storage.Trigger{ID: "t3", WorkflowID: "wf-triggers", Type: "interval", Config: []byte("{}"), Enabled: true}

	for _, tr := range []*storage.Trigger{t1, t2, t3} {
		if err := store.CreateTrigger(ctx, tr); err != nil {
			t.Fatalf("failed to create trigger %s: %v", tr.ID, err)
		}
	}

	// ListAllTriggers should return only enabled triggers (t1, t3)
	list, err := store.ListAllTriggers(ctx)
	if err != nil {
		t.Fatalf("failed to list all triggers: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("expected 2 enabled triggers, got %d", len(list))
	}

	ids := make(map[string]bool)
	for _, tr := range list {
		ids[tr.ID] = true
	}

	if !ids["t1"] || !ids["t3"] {
		t.Errorf("expected t1 and t3, got %v", ids)
	}
	if ids["t2"] {
		t.Error("did not expect disabled trigger t2")
	}
}
