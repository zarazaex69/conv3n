package main

import (
	"context"
	"fmt"
	"log"

	"github.com/zarazaex69/conv3n/internal/storage"
)

func main() {

	store, err := storage.NewSQLite("./conv3n.db")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Example 1: Create a new workflow execution
	workflowID := "example-workflow-001"
	executionID, err := store.CreateExecution(ctx, workflowID)
	if err != nil {
		log.Fatalf("Failed to create execution: %v", err)
	}
	fmt.Printf("Execution created: %s\n", executionID)

	// Example 2: Save node results during execution
	nodeResults := map[string][]byte{
		"http_request_1": []byte(`{"statusCode":200,"body":"Success"}`),
		"transform_1":    []byte(`{"output":{"userId":123,"name":"John"}}`),
		"condition_1":    []byte(`{"branch":"success","result":true}`),
	}

	for nodeID, result := range nodeResults {
		err = store.SaveNodeResult(ctx, executionID, nodeID, result)
		if err != nil {
			log.Fatalf("Failed to save node %s result: %v", nodeID, err)
		}
		fmt.Printf("Node %s result saved\n", nodeID)
	}

	// Example 3: Retrieve specific node result
	nodeResult, err := store.GetNodeResult(ctx, executionID, "http_request_1")
	if err != nil {
		log.Fatalf("Failed to get node result: %v", err)
	}
	fmt.Printf("Node result: %s\n", nodeResult)

	// Example 4: Mark execution as completed
	finalState := []byte(`{"status":"completed","totalNodes":3,"successNodes":3}`)
	err = store.UpdateExecutionStatus(ctx, executionID, storage.ExecutionStatusCompleted, finalState, nil)
	if err != nil {
		log.Fatalf("Failed to update execution status: %v", err)
	}
	fmt.Println("Execution marked as completed")

	// Example 5: Retrieve execution details
	exec, err := store.GetExecution(ctx, executionID)
	if err != nil {
		log.Fatalf("Failed to get execution: %v", err)
	}
	fmt.Printf("Execution status: %s\n", exec.Status)
	fmt.Printf("Started at: %s\n", exec.StartedAt)
	if exec.CompletedAt != nil {
		fmt.Printf("Completed at: %s\n", *exec.CompletedAt)
	}

	// Example 6: Create another execution (demonstrating history)
	executionID2, _ := store.CreateExecution(ctx, workflowID)
	errorMsg := "node timeout after 30s"
	failedState := []byte(`{"status":"failed","failedNode":"http_request_1"}`)
	store.UpdateExecutionStatus(ctx, executionID2, storage.ExecutionStatusFailed, failedState, &errorMsg)
	fmt.Printf("Second execution created and marked as failed: %s\n", executionID2)

	// Example 7: List execution history
	executions, err := store.ListExecutions(ctx, workflowID, 10)
	if err != nil {
		log.Fatalf("Failed to list executions: %v", err)
	}
	fmt.Printf("\nExecution History for workflow %s:\n", workflowID)
	for i, e := range executions {
		fmt.Printf("  %d. %s - Status: %s, Started: %s\n", i+1, e.ID, e.Status, e.StartedAt)
		if e.Error != nil {
			fmt.Printf("     Error: %s\n", *e.Error)
		}
	}

	fmt.Println("\nStorage example completed successfully!")
	fmt.Println("Database file created at: ./conv3n.db")
	fmt.Println("You can inspect it with: sqlite3 conv3n.db")
	fmt.Println("Try: SELECT * FROM workflow_executions;")
}
