package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zarazaex69/conv3n/internal/api"
	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

func newTriggerMux(t *testing.T) (*http.ServeMux, storage.Storage, *engine.TriggerManager) {
	store := newTestStorage(t)
	registry := engine.NewExecutionRegistry()
	workerPool := engine.NewWorkerPool(10)
	tm := engine.NewTriggerManager(store, t.TempDir(), registry, workerPool)

	handler := api.NewTriggerHandler(store, tm)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/triggers", handler.Create)
	mux.HandleFunc("GET /api/triggers", handler.List)
	mux.HandleFunc("GET /api/triggers/{id}", handler.Get)
	mux.HandleFunc("PUT /api/triggers/{id}", handler.Update)
	mux.HandleFunc("DELETE /api/triggers/{id}", handler.Delete)
	mux.HandleFunc("GET /api/triggers/{id}/executions", handler.ListExecutions)
	mux.HandleFunc("POST /api/webhooks/{id}", handler.HandleWebhook)

	return mux, store, tm
}

func TestTriggerAPI_Create(t *testing.T) {
	mux, store, _ := newTriggerMux(t)
	ctx := testCtx

	// Create workflow first
	wf := &storage.Workflow{ID: "wf-1", Name: "Test Workflow", Definition: []byte("{}")}
	store.CreateWorkflow(ctx, wf)

	// Create trigger request
	reqBody := api.CreateTriggerRequest{
		WorkflowID: "wf-1",
		Type:       "cron",
		Config:     map[string]interface{}{"schedule": "* * * * *"},
		Enabled:    true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/triggers", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created storage.Trigger
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if created.WorkflowID != "wf-1" {
		t.Errorf("expected workflow_id wf-1, got %s", created.WorkflowID)
	}
}

func TestTriggerAPI_Webhook(t *testing.T) {
	mux, store, _ := newTriggerMux(t)
	ctx := testCtx

	// Create workflow
	wfDef := map[string]interface{}{
		"id":    "wf-webhook",
		"nodes": []interface{}{},
		"edges": []interface{}{},
	}
	wfBytes, _ := json.Marshal(wfDef)
	store.CreateWorkflow(ctx, &storage.Workflow{ID: "wf-webhook", Name: "Webhook WF", Definition: wfBytes})

	// Create webhook trigger via API
	reqBody := api.CreateTriggerRequest{
		WorkflowID: "wf-webhook",
		Type:       "webhook",
		Config:     map[string]interface{}{},
		Enabled:    true,
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/triggers", bytes.NewReader(body))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	var created storage.Trigger
	json.NewDecoder(createRec.Body).Decode(&created)
	triggerID := created.ID

	// Fire webhook
	webhookReq := httptest.NewRequest(http.MethodPost, "/api/webhooks/"+triggerID, bytes.NewBufferString(`{"foo":"bar"}`))
	webhookReq.Header.Set("Content-Type", "application/json")
	webhookRec := httptest.NewRecorder()
	mux.ServeHTTP(webhookRec, webhookReq)

	if webhookRec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", webhookRec.Code, webhookRec.Body.String())
	}
}

func TestTriggerAPI_ListExecutions(t *testing.T) {
	mux, store, _ := newTriggerMux(t)
	ctx := testCtx

	triggerID := "tr-1"
	store.CreateTriggerExecution(ctx, &storage.TriggerExecution{
		ID:        "exec-1",
		TriggerID: triggerID,
		Status:    "success",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/triggers/"+triggerID+"/executions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var execs []storage.TriggerExecution
	json.NewDecoder(rec.Body).Decode(&execs)

	if len(execs) != 1 {
		t.Errorf("expected 1 execution, got %d", len(execs))
	}
}

func TestTriggerAPI_CRUD(t *testing.T) {
	mux, store, _ := newTriggerMux(t)
	ctx := testCtx

	// Create workflow
	wf := &storage.Workflow{ID: "wf-crud", Name: "CRUD Workflow", Definition: []byte("{}")}
	store.CreateWorkflow(ctx, wf)

	// 1. Create Trigger
	reqBody := api.CreateTriggerRequest{
		WorkflowID: "wf-crud",
		Type:       "cron",
		Config:     map[string]interface{}{"schedule": "* * * * *"},
		Enabled:    true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/triggers", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var created storage.Trigger
	json.NewDecoder(rec.Body).Decode(&created)
	triggerID := created.ID

	// 2. Get Trigger
	getReq := httptest.NewRequest(http.MethodGet, "/api/triggers/"+triggerID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Errorf("Get: expected status 200, got %d", getRec.Code)
	}

	// 3. Update Trigger
	reqBody.Enabled = false
	updateBody, _ := json.Marshal(reqBody)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/triggers/"+triggerID, bytes.NewReader(updateBody))
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Errorf("Update: expected status 200, got %d", updateRec.Code)
	}

	// Verify update
	getRec2 := httptest.NewRecorder()
	mux.ServeHTTP(getRec2, getReq) // Re-use get request
	var updated storage.Trigger
	json.NewDecoder(getRec2.Body).Decode(&updated)
	if updated.Enabled {
		t.Error("Update: expected enabled=false")
	}

	// 4. List Triggers
	listReq := httptest.NewRequest(http.MethodGet, "/api/triggers?workflow_id=wf-crud", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Errorf("List: expected status 200, got %d", listRec.Code)
	}
	var list []storage.Trigger
	json.NewDecoder(listRec.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("List: expected 1 trigger, got %d", len(list))
	}

	// 5. Delete Trigger
	delReq := httptest.NewRequest(http.MethodDelete, "/api/triggers/"+triggerID, nil)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Errorf("Delete: expected status 204, got %d", delRec.Code)
	}

	// Verify deletion
	getRec3 := httptest.NewRecorder()
	mux.ServeHTTP(getRec3, getReq)
	if getRec3.Code != http.StatusNotFound {
		t.Errorf("Get after Delete: expected 404, got %d", getRec3.Code)
	}
}
