package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zarazaex69/conv3n/internal/api"
	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

// version is set via ldflags during build
var Version = "dev"

// Server holds the server configuration
type Server struct {
	BlocksDir string
	Store     storage.Storage
	Registry  *engine.ExecutionRegistry
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Determine blocks directory
	blocksDir := os.Getenv("CONV3N_BLOCKS_DIR")
	if blocksDir == "" {
		cwd, _ := os.Getwd()
		blocksDir = filepath.Join(cwd, "pkg", "blocks")
	}

	// Initialize Storage
	store, err := storage.NewSQLite("conv3n.db")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	switch command {
	case "server":
		runServer(blocksDir, store)
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: conv3n run <workflow_file.json>")
			os.Exit(1)
		}
		runCLI(os.Args[2], blocksDir, store)
	case "version", "--version", "-v":
		fmt.Printf("conv3n %s\n", Version)
		return
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  conv3n server               Start the API server")
	fmt.Println("  conv3n run <workflow.json>  Run a workflow file once (CLI mode)")
	fmt.Println("  conv3n version              Show version information")
}

// --- Server Mode ---

func runServer(blocksDir string, store storage.Storage) {
	fmt.Println("Starting Conv3n API Server...")

	// Initialize execution registry for lifecycle management
	registry := engine.NewExecutionRegistry()

	// Initialize worker pool (limit to 20 concurrent workflows)
	workerPool := engine.NewWorkerPool(20)

	// Initialize trigger manager
	triggerManager := engine.NewTriggerManager(store, blocksDir, registry, workerPool)

	// Load existing triggers from storage
	if err := triggerManager.LoadTriggers(context.Background()); err != nil {
		log.Printf("Warning: failed to load triggers: %v", err)
	}
	defer triggerManager.StopAll()

	server := &Server{
		BlocksDir: blocksDir,
		Store:     store,
		Registry:  registry,
	}

	mux := http.NewServeMux()

	// Execution API
	mux.HandleFunc("POST /api/run", server.handleRun)

	// Workflow CRUD API
	wfHandler := api.NewWorkflowHandler(store)
	mux.HandleFunc("POST /api/workflows", wfHandler.Create)
	mux.HandleFunc("GET /api/workflows/{id}", wfHandler.Get)
	mux.HandleFunc("PUT /api/workflows/{id}", wfHandler.Update)
	mux.HandleFunc("DELETE /api/workflows/{id}", wfHandler.Delete)
	mux.HandleFunc("GET /api/workflows", wfHandler.List)

	// Trigger API
	triggerHandler := api.NewTriggerHandler(store, triggerManager)
	mux.HandleFunc("POST /api/triggers", triggerHandler.Create)
	mux.HandleFunc("GET /api/triggers/{id}", triggerHandler.Get)
	mux.HandleFunc("PUT /api/triggers/{id}", triggerHandler.Update)
	mux.HandleFunc("DELETE /api/triggers/{id}", triggerHandler.Delete)
	mux.HandleFunc("GET /api/triggers", triggerHandler.List)
	mux.HandleFunc("GET /api/triggers", triggerHandler.List)
	mux.HandleFunc("GET /api/triggers/{id}/executions", triggerHandler.ListExecutions)
	mux.HandleFunc("POST /api/webhooks/{id}", triggerHandler.HandleWebhook)

	// Execution history API
	execHandler := api.NewExecutionHandler(store)
	mux.HandleFunc("GET /api/workflows/{id}/executions", execHandler.ListByWorkflow)
	mux.HandleFunc("GET /api/executions/{id}", execHandler.Get)
	mux.HandleFunc("GET /api/executions/{id}/nodes/{nodeId}", execHandler.GetNodeResult)

	// Lifecycle API (stop, restart)
	lifecycleHandler := api.NewLifecycleHandler(store, registry, blocksDir)
	mux.HandleFunc("POST /api/executions/{id}/stop", lifecycleHandler.StopExecution)
	mux.HandleFunc("POST /api/executions/{id}/restart", lifecycleHandler.RestartExecution)
	mux.HandleFunc("POST /api/executions/batch/stop", lifecycleHandler.BatchStopExecutions)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		// Return worker pool stats
		stats := workerPool.Stats()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "OK",
			"workers": stats,
		})
	})

	fmt.Printf("Listening on http://localhost:8080\n")
	fmt.Printf("Blocks loaded from: %s\n", blocksDir)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- httpServer.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case sig := <-shutdown:
		fmt.Printf("\nReceived signal %v, shutting down gracefully...\n", sig)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
			httpServer.Close()
		}

		fmt.Println("Server stopped")
	}
}

type RunRequest struct {
	Workflow engine.Workflow `json:"workflow"`
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	if r.Method == "OPTIONS" {
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON: "+err.Error(), 400)
		return
	}

	ctx := engine.NewExecutionContext(req.Workflow.ID)
	runner := engine.NewWorkflowRunner(ctx, s.BlocksDir, s.Store, s.Registry)

	fmt.Printf("New Job: %s\n", req.Workflow.Name)

	// Create cancellable context for execution
	execCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Note: We would register execution ID here if we had it before running
	// For now, the runner creates the execution ID internally

	if err := runner.Run(execCtx, req.Workflow); err != nil {
		http.Error(w, "Execution Failed: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"results": ctx.Results,
	})
}

func enableCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// --- CLI Mode ---

func runCLI(filePath string, blocksDir string, store storage.Storage) {
	fmt.Printf("Reading workflow from: %s\n", filePath)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	var workflow engine.Workflow
	if err := json.Unmarshal(data, &workflow); err != nil {
		log.Fatalf("Failed to parse workflow JSON: %v", err)
	}

	fmt.Println("Starting conv3n (Bunock) Engine...")
	fmt.Printf("Using Blocks Directory: %s\n", blocksDir)

	ctx := engine.NewExecutionContext(workflow.ID)
	// CLI mode doesn't need lifecycle management, pass nil registry
	runner := engine.NewWorkflowRunner(ctx, blocksDir, store, nil)

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

		// Handle standard blocks (http_request, etc.) - they have "status" and "data"
		if status, hasStatus := resMap["status"]; hasStatus {
			data := resMap["data"]
			fmt.Printf("Block [%s]: Status %v\n", blockID, status)
			fmt.Printf("Data: %+v\n\n", data)
		} else if success, hasSuccess := resMap["success"]; hasSuccess {
			// Handle custom/code blocks - they have "success", "data", and "executionTime"
			data := resMap["data"]
			execTime := resMap["executionTime"]
			fmt.Printf("Block [%s]: Success %v (%.2fms)\n", blockID, success, execTime)
			fmt.Printf("Data: %+v\n\n", data)
		} else {
			// Fallback for unknown structure
			fmt.Printf("Block [%s]: %+v\n\n", blockID, resMap)
		}
	}

}

// btw i want t suicide
