package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/zarazaex69/conv3n/internal/observability"
	"github.com/zarazaex69/conv3n/pkg/conv3n"
)

type LoggingHandler struct{}

func (h *LoggingHandler) OnExecutionStart(execID, workflowID string) {
	slog.Info("execution started",
		slog.String("execution_id", execID),
		slog.String("workflow_id", workflowID),
	)
}

func (h *LoggingHandler) OnExecutionComplete(execID string, err error) {
	if err != nil {
		slog.Error("execution failed",
			slog.String("execution_id", execID),
			slog.Any("error", err),
		)
	} else {
		slog.Info("execution completed",
			slog.String("execution_id", execID),
		)
	}
}

func (h *LoggingHandler) OnExecutionStop(execID string) {
	slog.Info("execution stopped", slog.String("execution_id", execID))
}

func (h *LoggingHandler) OnNodeExecute(execID, nodeID string, result map[string]interface{}) {
	slog.Info("node executed",
		slog.String("execution_id", execID),
		slog.String("node_id", nodeID),
	)
}

func main() {
	cfg := conv3n.DefaultConfig()
	cfg.BlocksDir = "pkg/blocks"
	cfg.StoragePath = "production.db"
	cfg.WorkerPoolSize = 8
	cfg.LogLevel = slog.LevelInfo
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

	go startHTTPServer(runtime)

	wf := conv3n.NewWorkflow("production_example", "Production Ready Example")

	wf.AddNode(&conv3n.Node{
		ID:       "fetch_data",
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
		Source:       "fetch_data",
		Target:       "transform",
		SourceHandle: "default",
		TargetHandle: "main",
	})

	slog.Info("executing workflow")
	handle, err := runtime.Execute(ctx, wf, nil)
	if err != nil {
		log.Fatalf("Failed to execute workflow: %v", err)
	}

	slog.Info("workflow submitted", slog.String("execution_id", handle.ID()))

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := handle.Wait(waitCtx); err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	status, err := handle.Status(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	slog.Info("execution completed", slog.String("status", string(status.Status)))

	runtime.WaitForShutdown(ctx)
}

func startHTTPServer(runtime *conv3n.Runtime) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := runtime.Health(r.Context())
		w.Header().Set("Content-Type", "application/json")

		status := health["status"].(string)
		if status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(health)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := runtime.Metrics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	})

	prometheusExporter := observability.NewPrometheusExporter(observability.GetMetrics())
	mux.HandleFunc("/metrics/prometheus", prometheusExporter.Handler())

	mux.HandleFunc("/traces", func(w http.ResponseWriter, r *http.Request) {
		traces := runtime.Traces()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(traces)
	})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	slog.Info("http server starting", slog.String("addr", server.Addr))

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("http server failed", slog.Any("error", err))
	}
}
