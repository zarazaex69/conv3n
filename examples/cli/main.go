package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: cli_runner <workflow.json>")
		os.Exit(1)
	}

	filePath := os.Args[1]
	fmt.Printf("Reading workflow from: %s\n", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	var workflow engine.Workflow
	if err := json.Unmarshal(data, &workflow); err != nil {
		log.Fatalf("Failed to parse workflow JSON: %v", err)
	}

	blocksDir := os.Getenv("CONV3N_BLOCKS_DIR")
	if blocksDir == "" {
		cwd, _ := os.Getwd()
		blocksDir = filepath.Join(cwd, "pkg", "blocks")
	}

	fmt.Println("Starting conv3n Engine...")
	fmt.Printf("Using Blocks Directory: %s\n", blocksDir)

	blockRegistry := engine.NewBlockRegistry(blocksDir)
	if err := blockRegistry.LoadFromDirectory(blocksDir); err != nil {
		log.Printf("Warning: failed to load block manifests: %v", err)
	} else {
		manifests := blockRegistry.List()
		fmt.Printf("Loaded %d blocks\n", len(manifests))
	}

	store, err := storage.NewSQLite("conv3n.db")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	workerPool, err := engine.NewWorkerPool(4, "bun", "pkg/bunock/worker_server.ts")
	if err != nil {
		log.Fatalf("Failed to initialize worker pool: %v", err)
	}
	defer workerPool.Shutdown()

	registry := engine.NewExecutionRegistry()
	ctx := engine.NewExecutionContext(workflow.ID)
	runner := engine.NewWorkflowRunner(ctx, workerPool, store, registry, blockRegistry)

	fmt.Printf("Running Workflow: %s\n", workflow.Name)

	execCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := runner.Run(execCtx, workflow); err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	fmt.Println("\n--- Execution Results ---")
	for blockID, result := range ctx.Results {
		resMap, ok := result.(map[string]interface{})
		if !ok {
			fmt.Printf("Block [%s]: %+v\n\n", blockID, result)
			continue
		}

		if status, hasStatus := resMap["status"]; hasStatus {
			data := resMap["data"]
			fmt.Printf("Block [%s]: Status %v\n", blockID, status)
			fmt.Printf("Data: %+v\n\n", data)
		} else if success, hasSuccess := resMap["success"]; hasSuccess {
			data := resMap["data"]
			execTime := resMap["executionTime"]
			fmt.Printf("Block [%s]: Success %v (%.2fms)\n", blockID, success, execTime)
			fmt.Printf("Data: %+v\n\n", data)
		} else {
			fmt.Printf("Block [%s]: %+v\n\n", blockID, resMap)
		}
	}
}
