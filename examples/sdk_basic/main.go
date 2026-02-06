package main

import (
	"context"
	"log"
	"time"

	"github.com/zarazaex69/conv3n/pkg/conv3n"
)

type LoggingHandler struct{}

func (h *LoggingHandler) OnExecutionStart(execID, workflowID string) {
	log.Printf("[START] Execution %s for workflow %s", execID, workflowID)
}

func (h *LoggingHandler) OnExecutionComplete(execID string, err error) {
	if err != nil {
		log.Printf("[ERROR] Execution %s failed: %v", execID, err)
	} else {
		log.Printf("[COMPLETE] Execution %s finished successfully", execID)
	}
}

func (h *LoggingHandler) OnExecutionStop(execID string) {
	log.Printf("[STOP] Execution %s was stopped", execID)
}

func (h *LoggingHandler) OnNodeExecute(execID, nodeID string, result map[string]interface{}) {
	log.Printf("[NODE] %s executed in %s", nodeID, execID)
}

func main() {
	cfg := conv3n.DefaultConfig()
	cfg.BlocksDir = "pkg/blocks"
	cfg.StoragePath = "examples_sdk.db"
	cfg.WorkerPoolSize = 5
	cfg.EventHandler = &LoggingHandler{}

	runtime, err := conv3n.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()
	if err := runtime.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer runtime.Stop(ctx)

	wf := conv3n.NewWorkflow("sdk_example", "SDK Basic Example")

	wf.AddNode(&conv3n.Node{
		ID:       "http_fetch",
		Type:     "std/http_request",
		Position: conv3n.Position{X: 100, Y: 100},
		Config: map[string]interface{}{
			"url":    "https://jsonplaceholder.typicode.com/users/1",
			"method": "GET",
		},
	})

	wf.AddNode(&conv3n.Node{
		ID:       "transform",
		Type:     "std/transform",
		Position: conv3n.Position{X: 350, Y: 100},
		Config: map[string]interface{}{
			"code": "return { username: input.data.username, email: input.data.email }",
		},
	})

	wf.AddEdge(&conv3n.Edge{
		ID:           "e1",
		Source:       "http_fetch",
		Target:       "transform",
		SourceHandle: "default",
		TargetHandle: "main",
	})

	log.Println("Executing workflow...")
	handle, err := runtime.Execute(ctx, wf, nil)
	if err != nil {
		log.Fatalf("Failed to execute workflow: %v", err)
	}

	log.Printf("Execution started: %s", handle.ID())

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := handle.Wait(waitCtx); err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	status, err := handle.Status(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	log.Printf("Final status: %s", status.Status)

	result, err := handle.GetNodeResult(ctx, "transform")
	if err != nil {
		log.Printf("Warning: Could not get node result: %v", err)
	} else {
		log.Printf("Transform result: %+v", result)
	}

	executions, err := runtime.ListExecutions(ctx, wf.ID, 10)
	if err != nil {
		log.Printf("Warning: Could not list executions: %v", err)
	} else {
		log.Printf("Total executions for this workflow: %d", len(executions))
	}
}
