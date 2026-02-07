package main

import (
	"context"
	"fmt"
	"log"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type ConsoleEventListener struct{}

func (l *ConsoleEventListener) OnEvent(event engine.Event) {
	switch event.Type {
	case engine.EventTypeExecutionStart:
		data := event.Data.(engine.ExecutionStartData)
		fmt.Printf("[%s] ▶ Execution started: %s (workflow: %s)\n",
			event.Timestamp.Format("15:04:05"),
			event.ExecutionID,
			event.WorkflowID)
		fmt.Printf("  Start nodes: %v\n", data.StartNodes)

	case engine.EventTypeExecutionComplete:
		data := event.Data.(engine.ExecutionCompleteData)
		fmt.Printf("[%s] ✓ Execution completed: %s (duration: %s)\n",
			event.Timestamp.Format("15:04:05"),
			event.ExecutionID,
			data.Duration)

	case engine.EventTypeExecutionError:
		fmt.Printf("[%s] ✗ Execution failed: %s\n",
			event.Timestamp.Format("15:04:05"),
			event.ExecutionID)
		if event.Error != nil {
			fmt.Printf("  Error: %v\n", event.Error)
		}

	case engine.EventTypeNodeStart:
		data := event.Data.(engine.NodeStartData)
		fmt.Printf("[%s]   → Node started: %s (type: %s)\n",
			event.Timestamp.Format("15:04:05"),
			event.NodeID,
			data.NodeType)

	case engine.EventTypeNodeComplete:
		data := event.Data.(engine.NodeCompleteData)
		fmt.Printf("[%s]   ✓ Node completed: %s (duration: %s, port: %s)\n",
			event.Timestamp.Format("15:04:05"),
			event.NodeID,
			data.Duration,
			data.Port)

	case engine.EventTypeNodeError:
		fmt.Printf("[%s]   ✗ Node failed: %s\n",
			event.Timestamp.Format("15:04:05"),
			event.NodeID)
		if event.Error != nil {
			fmt.Printf("    Error: %v\n", event.Error)
		}
	}
}

func main() {
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	workflow := &engine.Workflow{
		ID:   "example-workflow",
		Name: "Example Workflow with Events",
		Nodes: map[string]engine.Node{
			"delay1": {
				ID:   "delay1",
				Type: engine.NodeTypeDelay,
				Config: map[string]interface{}{
					"duration": 100,
					"unit":     "ms",
				},
			},
			"delay2": {
				ID:   "delay2",
				Type: engine.NodeTypeDelay,
				Config: map[string]interface{}{
					"duration": 150,
					"unit":     "ms",
				},
			},
		},
		Edges: []engine.Edge{
			{ID: "e1", Source: "delay1", Target: "delay2"},
		},
	}

	workerPool, err := engine.NewWorkerPool(4, "bun", "pkg/bunock/worker_server.ts")
	if err != nil {
		log.Fatalf("Failed to initialize worker pool: %v", err)
	}
	defer workerPool.Shutdown()

	blockRegistry := engine.NewBlockRegistry("pkg/blocks")
	if err := blockRegistry.LoadFromDirectory("pkg/blocks"); err != nil {
		log.Printf("Warning: failed to load block manifests: %v", err)
	}

	registry := engine.NewExecutionRegistry()
	execCtx := engine.NewExecutionContext(workflow.ID)
	runner := engine.NewWorkflowRunner(execCtx, workerPool, store, registry, blockRegistry)

	fmt.Println("Starting workflow execution...")
	fmt.Println()

	ctx := context.Background()
	if err := runner.Run(ctx, *workflow); err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	fmt.Println()
	fmt.Println("Workflow execution completed successfully!")
}
