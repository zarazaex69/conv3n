package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/api"
	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type E2EServer struct {
	mux        *http.ServeMux
	store      storage.Storage
	registry   *engine.ExecutionRegistry
	workerPool *engine.WorkerPool
	triggerMgr *engine.TriggerManager
	blocksDir  string
	httpServer *httptest.Server
}

func setupE2EServer(t *testing.T) *E2EServer {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "e2e.db")

	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	blocksDir := filepath.Join(filepath.Dir(filepath.Dir(cwd)), "pkg", "blocks")

	registry := engine.NewExecutionRegistry()
	workerPool := engine.NewWorkerPool(10)
	triggerMgr := engine.NewTriggerManager(store, blocksDir, registry, workerPool)

	mux := http.NewServeMux()

	wfHandler := api.NewWorkflowHandler(store)
	mux.HandleFunc("POST /api/workflows", wfHandler.Create)
	mux.HandleFunc("GET /api/workflows/{id}", wfHandler.Get)
	mux.HandleFunc("PUT /api/workflows/{id}", wfHandler.Update)
	mux.HandleFunc("DELETE /api/workflows/{id}", wfHandler.Delete)
	mux.HandleFunc("GET /api/workflows", wfHandler.List)

	triggerHandler := api.NewTriggerHandler(store, triggerMgr)
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

	lifecycleHandler := api.NewLifecycleHandler(store, registry, blocksDir)
	mux.HandleFunc("POST /api/executions/{id}/stop", lifecycleHandler.StopExecution)
	mux.HandleFunc("POST /api/executions/{id}/restart", lifecycleHandler.RestartExecution)
	mux.HandleFunc("POST /api/executions/batch/stop", lifecycleHandler.BatchStopExecutions)

	mux.HandleFunc("POST /api/run", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Workflow engine.Workflow `json:"workflow"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad JSON: "+err.Error(), 400)
			return
		}

		ctx := engine.NewExecutionContext(req.Workflow.ID)
		runner := engine.NewWorkflowRunner(ctx, blocksDir, store, registry)

		execCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
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
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		stats := workerPool.Stats()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "OK",
			"workers": stats,
		})
	})

	httpServer := httptest.NewServer(mux)

	t.Cleanup(func() {
		httpServer.Close()
		triggerMgr.StopAll()
		store.Close()
	})

	return &E2EServer{
		mux:        mux,
		store:      store,
		registry:   registry,
		workerPool: workerPool,
		triggerMgr: triggerMgr,
		blocksDir:  blocksDir,
		httpServer: httpServer,
	}
}

func TestE2E_FullStack(t *testing.T) {
	srv := setupE2EServer(t)
	client := srv.httpServer.Client()
	baseURL := srv.httpServer.URL

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("failed to decode health response: %v", err)
		}

		if health["status"] != "OK" {
			t.Errorf("expected status OK, got %v", health["status"])
		}
	})

	t.Run("CreateWorkflow", func(t *testing.T) {
		workflow := engine.Workflow{
			Name: "E2E Test Workflow",
			Nodes: map[string]engine.Node{
				"start": {
					ID:   "start",
					Type: "std/transform",
					Config: map[string]interface{}{
						"expression": "input.value * 2",
					},
				},
			},
			Edges: []engine.Edge{},
		}

		body, _ := json.Marshal(workflow)
		resp, err := client.Post(baseURL+"/api/workflows", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}

		var created engine.Workflow
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if created.ID == "" {
			t.Error("expected non-empty workflow ID")
		}
		if created.Name != workflow.Name {
			t.Errorf("expected name %s, got %s", workflow.Name, created.Name)
		}
	})

	t.Run("ExecuteWorkflow", func(t *testing.T) {
		workflow := engine.Workflow{
			ID:   "exec-test-wf",
			Name: "Execution Test",
			Nodes: map[string]engine.Node{
				"delay": {
					ID:   "delay",
					Type: "std/delay",
					Config: map[string]interface{}{
						"duration": 10,
					},
				},
			},
			Edges: []engine.Edge{},
		}

		runReq := map[string]interface{}{
			"workflow": workflow,
		}

		body, _ := json.Marshal(runReq)
		resp, err := client.Post(baseURL+"/api/run", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("failed to execute workflow: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes := new(bytes.Buffer)
			bodyBytes.ReadFrom(resp.Body)
			t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, bodyBytes.String())
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode execution result: %v", err)
		}

		if result["status"] != "success" {
			t.Errorf("expected status success, got %v", result["status"])
		}

		results, ok := result["results"].(map[string]interface{})
		if !ok {
			t.Fatal("expected results map")
		}

		delayResult, exists := results["delay"]
		if !exists {
			t.Fatal("expected delay node result")
		}

		if delayResult == nil {
			t.Fatal("delay result is nil")
		}
	})

	t.Run("WorkflowCRUDFlow", func(t *testing.T) {
		workflow := engine.Workflow{
			Name:  "CRUD Test",
			Nodes: map[string]engine.Node{},
			Edges: []engine.Edge{},
		}

		body, _ := json.Marshal(workflow)
		resp, err := client.Post(baseURL+"/api/workflows", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
		defer resp.Body.Close()

		var created engine.Workflow
		json.NewDecoder(resp.Body).Decode(&created)
		workflowID := created.ID

		getResp, err := client.Get(fmt.Sprintf("%s/api/workflows/%s", baseURL, workflowID))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", getResp.StatusCode)
		}

		created.Name = "Updated CRUD Test"
		updateBody, _ := json.Marshal(created)
		updateReq, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/workflows/%s", baseURL, workflowID), bytes.NewReader(updateBody))
		updateResp, err := client.Do(updateReq)
		if err != nil {
			t.Fatalf("update failed: %v", err)
		}
		defer updateResp.Body.Close()

		if updateResp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", updateResp.StatusCode)
		}

		listResp, err := client.Get(baseURL + "/api/workflows")
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		defer listResp.Body.Close()

		var workflows []map[string]interface{}
		json.NewDecoder(listResp.Body).Decode(&workflows)

		found := false
		for _, wf := range workflows {
			if wf["id"] == workflowID {
				found = true
				if wf["name"] != "Updated CRUD Test" {
					t.Errorf("expected updated name, got %v", wf["name"])
				}
				break
			}
		}

		if !found {
			t.Error("workflow not found in list")
		}

		deleteReq, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/workflows/%s", baseURL, workflowID), nil)
		deleteResp, err := client.Do(deleteReq)
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		defer deleteResp.Body.Close()

		if deleteResp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", deleteResp.StatusCode)
		}

		verifyResp, _ := client.Get(fmt.Sprintf("%s/api/workflows/%s", baseURL, workflowID))
		defer verifyResp.Body.Close()

		if verifyResp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404 for deleted workflow, got %d", verifyResp.StatusCode)
		}
	})

	t.Run("TriggerLifecycle", func(t *testing.T) {
		workflow := engine.Workflow{
			Name:  "Trigger Test Workflow",
			Nodes: map[string]engine.Node{},
			Edges: []engine.Edge{},
		}

		wfBody, _ := json.Marshal(workflow)
		wfResp, err := client.Post(baseURL+"/api/workflows", "application/json", bytes.NewReader(wfBody))
		if err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}
		defer wfResp.Body.Close()

		var createdWF engine.Workflow
		json.NewDecoder(wfResp.Body).Decode(&createdWF)

		trigger := map[string]interface{}{
			"workflow_id": createdWF.ID,
			"type":        "webhook",
			"config": map[string]interface{}{
				"path": "/test-webhook",
			},
			"enabled": true,
		}

		triggerBody, _ := json.Marshal(trigger)
		triggerResp, err := client.Post(baseURL+"/api/triggers", "application/json", bytes.NewReader(triggerBody))
		if err != nil {
			t.Fatalf("failed to create trigger: %v", err)
		}
		defer triggerResp.Body.Close()

		if triggerResp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", triggerResp.StatusCode)
		}

		var createdTrigger map[string]interface{}
		json.NewDecoder(triggerResp.Body).Decode(&createdTrigger)

		triggerID, ok := createdTrigger["id"].(string)
		if !ok || triggerID == "" {
			t.Fatal("expected valid trigger ID")
		}

		getTriggerResp, err := client.Get(fmt.Sprintf("%s/api/triggers/%s", baseURL, triggerID))
		if err != nil {
			t.Fatalf("failed to get trigger: %v", err)
		}
		defer getTriggerResp.Body.Close()

		if getTriggerResp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", getTriggerResp.StatusCode)
		}

		deleteTriggerReq, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/triggers/%s", baseURL, triggerID), nil)
		deleteTriggerResp, err := client.Do(deleteTriggerReq)
		if err != nil {
			t.Fatalf("failed to delete trigger: %v", err)
		}
		defer deleteTriggerResp.Body.Close()

		if deleteTriggerResp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", deleteTriggerResp.StatusCode)
		}
	})

	t.Run("ExecutionHistory", func(t *testing.T) {
		workflow := engine.Workflow{
			Name: "History Test",
			Nodes: map[string]engine.Node{
				"delay": {
					ID:   "delay",
					Type: "std/delay",
					Config: map[string]interface{}{
						"duration": 5,
					},
				},
			},
			Edges: []engine.Edge{},
		}

		wfBody, _ := json.Marshal(workflow)
		wfResp, err := client.Post(baseURL+"/api/workflows", "application/json", bytes.NewReader(wfBody))
		if err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}
		defer wfResp.Body.Close()

		var createdWF engine.Workflow
		json.NewDecoder(wfResp.Body).Decode(&createdWF)

		for i := 0; i < 3; i++ {
			runReq := map[string]interface{}{
				"workflow": createdWF,
			}
			runBody, _ := json.Marshal(runReq)
			runResp, err := client.Post(baseURL+"/api/run", "application/json", bytes.NewReader(runBody))
			if err != nil {
				t.Fatalf("execution %d failed: %v", i, err)
			}
			runResp.Body.Close()
			time.Sleep(10 * time.Millisecond)
		}

		historyResp, err := client.Get(fmt.Sprintf("%s/api/workflows/%s/executions", baseURL, createdWF.ID))
		if err != nil {
			t.Fatalf("failed to get execution history: %v", err)
		}
		defer historyResp.Body.Close()

		var executions []map[string]interface{}
		json.NewDecoder(historyResp.Body).Decode(&executions)

		if len(executions) < 3 {
			t.Errorf("expected at least 3 executions, got %d", len(executions))
		}
	})
}
