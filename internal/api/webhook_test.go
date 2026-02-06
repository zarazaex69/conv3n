package api_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestWebhookTrigger(t *testing.T) {

	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	// 2. Setup Dependencies

	// (or we can use a simple workflow that doesn't fail)
	tmpDir := t.TempDir()
	blocksDir := filepath.Join(tmpDir, "blocks")
	os.MkdirAll(blocksDir, 0755)

	// Create dummy std/condition block
	conditionDir := filepath.Join(blocksDir, "std")
	os.MkdirAll(conditionDir, 0755)
	conditionFile := filepath.Join(conditionDir, "condition.ts")

	dummyScript := `
	console.log(JSON.stringify({
		status: "success",
		data: { result: true },
		port: "true"
	}));
	`
	os.WriteFile(conditionFile, []byte(dummyScript), 0644)

	registry := engine.NewExecutionRegistry()
	workerPool := engine.NewWorkerPool(5)
	triggerManager := engine.NewTriggerManager(store, blocksDir, registry, workerPool)

	handler := api.NewTriggerHandler(store, triggerManager)

	// 3. Create Workflow
	wf := &storage.Workflow{
		ID:   "wf-1",
		Name: "Test Workflow",
		Definition: []byte(`{
			"id": "wf-1",
			"nodes": {
				"start": { "id": "start", "type": "std/condition", "config": { "expression": "true" } }
			},
			"edges": []
		}`),
	}
	if err := store.CreateWorkflow(context.Background(), wf); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// 4. Create Webhook Trigger
	config := map[string]interface{}{}
	configBytes, _ := json.Marshal(config)
	trigger := &storage.Trigger{
		ID:         "tr-1",
		WorkflowID: "wf-1",
		Type:       "webhook",
		Config:     configBytes,
		Enabled:    true,
	}
	if err := store.CreateTrigger(context.Background(), trigger); err != nil {
		t.Fatalf("Failed to create trigger: %v", err)
	}

	// Register trigger in manager (LoadTriggers would do this)

	// Actually Fire calls GetTrigger from DB to get workflow ID, but it also checks if trigger exists in manager?

	// "triggerRunner, exists := tm.GetTrigger(triggerID)" -> Yes, it checks manager.

	// We can use LoadTriggers or manually register.
	if err := triggerManager.LoadTriggers(context.Background()); err != nil {
		t.Fatalf("Failed to load triggers: %v", err)
	}

	// 5. Perform Request
	payload := map[string]interface{}{"foo": "bar"}
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/webhooks/tr-1", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// We need to route it properly or just call the handler method directly

	// But we need path value "id" which Go 1.22+ handles in ServeMux.

	// We can use a mux to route it.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/webhooks/{id}", handler.HandleWebhook)
	mux.ServeHTTP(w, req)

	// 6. Verify Response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 7. Verify Execution Started

	time.Sleep(100 * time.Millisecond)

	executions, err := store.ListTriggerExecutions(context.Background(), "tr-1", 10)
	if err != nil {
		t.Fatalf("Failed to list trigger executions: %v", err)
	}

	if len(executions) == 0 {
		t.Fatal("Expected trigger execution to be created, found 0")
	}

	exec := executions[0]
	if exec.Status != "success" && exec.Status != "running" {
		t.Errorf("Expected status success or running, got %s (error: %v)", exec.Status, exec.Error)
	}

	// Verify Payload was saved
	if exec.Payload == nil {
		t.Error("Expected payload to be saved, got nil")
	} else {

		// The payload saved is the constructed map with headers, body, etc.

		var savedPayload map[string]interface{}
		if err := json.Unmarshal(exec.Payload, &savedPayload); err != nil {
			t.Errorf("Failed to parse saved payload: %v", err)
		}

		body, ok := savedPayload["body"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected body in payload, got %v", savedPayload["body"])
		} else {
			if body["foo"] != "bar" {
				t.Errorf("Expected body.foo = bar, got %v", body["foo"])
			}
		}
	}
}
