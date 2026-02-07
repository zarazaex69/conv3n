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

var Version = "dev"

type Server struct {
	BlocksDir     string
	Store         storage.Storage
	Registry      *engine.ExecutionRegistry
	BlockRegistry *engine.BlockRegistry
	WorkerPool    *engine.WorkerPool
}

func main() {
	blocksDir := os.Getenv("CONV3N_BLOCKS_DIR")
	if blocksDir == "" {
		cwd, _ := os.Getwd()
		blocksDir = filepath.Join(cwd, "pkg", "blocks")
	}

	store, err := storage.NewSQLite("conv3n.db")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	fmt.Println("Starting Conv3n Reference Server...")
	fmt.Printf("Version: %s\n", Version)

	blockRegistry := engine.NewBlockRegistry(blocksDir)
	if err := blockRegistry.LoadFromDirectory(blocksDir); err != nil {
		log.Printf("Warning: failed to load block manifests: %v", err)
	} else {
		manifests := blockRegistry.List()
		fmt.Printf("Loaded %d blocks\n", len(manifests))
	}

	registry := engine.NewExecutionRegistry()

	workerPool, err := engine.NewWorkerPool(4, "bun", "pkg/bunock/worker_server.ts")
	if err != nil {
		log.Fatalf("Failed to initialize worker pool: %v", err)
	}
	defer workerPool.Shutdown()

	triggerManager := engine.NewTriggerManager(store, workerPool, registry, blockRegistry)
	if err := triggerManager.LoadTriggers(context.Background()); err != nil {
		log.Printf("Warning: failed to load triggers: %v", err)
	}
	defer triggerManager.StopAll()

	server := &Server{
		BlocksDir:     blocksDir,
		Store:         store,
		Registry:      registry,
		BlockRegistry: blockRegistry,
		WorkerPool:    workerPool,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/run", server.handleRun)

	wfHandler := api.NewWorkflowHandler(store)
	mux.HandleFunc("POST /api/workflows", wfHandler.Create)
	mux.HandleFunc("GET /api/workflows/{id}", wfHandler.Get)
	mux.HandleFunc("PUT /api/workflows/{id}", wfHandler.Update)
	mux.HandleFunc("DELETE /api/workflows/{id}", wfHandler.Delete)
	mux.HandleFunc("GET /api/workflows", wfHandler.List)

	triggerHandler := api.NewTriggerHandler(store, triggerManager)
	mux.HandleFunc("POST /api/triggers", triggerHandler.Create)
	mux.HandleFunc("GET /api/triggers/{id}", triggerHandler.Get)
	mux.HandleFunc("PUT /api/triggers/{id}", triggerHandler.Update)
	mux.HandleFunc("DELETE /api/triggers/{id}", triggerHandler.Delete)
	mux.HandleFunc("GET /api/triggers", triggerHandler.List)
	mux.HandleFunc("GET /api/triggers/{id}/executions", triggerHandler.ListExecutions)
	mux.HandleFunc("POST /api/webhooks/{id}", triggerHandler.HandleWebhook)

	execHandler := api.NewExecutionHandler(store)
	mux.HandleFunc("GET /api/workflows/{id}/executions", execHandler.ListByWorkflow)
	mux.HandleFunc("GET /api/executions/{id}", execHandler.Get)
	mux.HandleFunc("GET /api/executions/{id}/nodes/{nodeId}", execHandler.GetNodeResult)

	lifecycleHandler := api.NewLifecycleHandler(store, registry, workerPool, blockRegistry)
	mux.HandleFunc("POST /api/executions/{id}/stop", lifecycleHandler.StopExecution)
	mux.HandleFunc("POST /api/executions/{id}/restart", lifecycleHandler.RestartExecution)
	mux.HandleFunc("POST /api/executions/batch/stop", lifecycleHandler.BatchStopExecutions)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
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
	runner := engine.NewWorkflowRunner(ctx, s.WorkerPool, s.Store, s.Registry, s.BlockRegistry)

	fmt.Printf("New Job: %s\n", req.Workflow.Name)

	execCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

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
