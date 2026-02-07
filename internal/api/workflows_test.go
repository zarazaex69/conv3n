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

func newWorkflowMux(t *testing.T) (*http.ServeMux, storage.Storage) {
	store := newTestStorage(t)
	handler := api.NewWorkflowHandler(store)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/workflows", handler.Create)
	mux.HandleFunc("GET /api/workflows", handler.List)
	mux.HandleFunc("GET /api/workflows/{id}", handler.Get)
	mux.HandleFunc("PUT /api/workflows/{id}", handler.Update)
	mux.HandleFunc("DELETE /api/workflows/{id}", handler.Delete)

	return mux, store
}

func TestWorkflowAPI_CreateAndGet(t *testing.T) {
	mux, _ := newWorkflowMux(t)

	wf := engine.Workflow{
		Name:  "Test Workflow",
		Nodes: map[string]engine.Node{},
		Edges: []engine.Edge{},
	}

	body, err := json.Marshal(wf)
	if err != nil {
		t.Fatalf("failed to marshal workflow: %v", err)
	}

	// Create workflow
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var created engine.Workflow
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	if created.ID == "" {
		t.Fatalf("expected non-empty workflow ID")
	}

	// Get workflow by ID
	getReq := httptest.NewRequest(http.MethodGet, "/api/workflows/"+created.ID, nil)
	getRec := httptest.NewRecorder()

	mux.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRec.Code)
	}

	var fetched engine.Workflow
	if err := json.NewDecoder(getRec.Body).Decode(&fetched); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}

	if fetched.ID != created.ID {
		t.Fatalf("expected ID %q, got %q", created.ID, fetched.ID)
	}

	if fetched.Name != wf.Name {
		t.Fatalf("expected name %q, got %q", wf.Name, fetched.Name)
	}
}

func TestWorkflowAPI_ListAndDelete(t *testing.T) {
	mux, _ := newWorkflowMux(t)

	// Create first workflow
	wf1 := engine.Workflow{Name: "WF1"}
	body1, err := json.Marshal(wf1)
	if err != nil {
		t.Fatalf("failed to marshal wf1: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewReader(body1))
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec1.Code)
	}

	var created1 engine.Workflow
	if err := json.NewDecoder(rec1.Body).Decode(&created1); err != nil {
		t.Fatalf("failed to decode wf1 response: %v", err)
	}

	// Create second workflow
	wf2 := engine.Workflow{Name: "WF2"}
	body2, err := json.Marshal(wf2)
	if err != nil {
		t.Fatalf("failed to marshal wf2: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec2.Code)
	}

	// List workflows
	listReq := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRec.Code)
	}

	var listResp struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}

	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	if len(listResp.Items) != 2 {
		t.Fatalf("expected 2 workflows in list, got %d", len(listResp.Items))
	}

	// Delete first workflow
	delReq := httptest.NewRequest(http.MethodDelete, "/api/workflows/"+created1.ID, nil)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, delRec.Code)
	}

	// Ensure deleted workflow is no longer returned by Get
	getReq := httptest.NewRequest(http.MethodGet, "/api/workflows/"+created1.ID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for deleted workflow, got %d", http.StatusNotFound, getRec.Code)
	}
}

func TestWorkflowAPI_Create_InvalidJSON(t *testing.T) {
	mux, _ := newWorkflowMux(t)

	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewBufferString("not-json"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestWorkflowAPI_Update(t *testing.T) {
	mux, _ := newWorkflowMux(t)

	// Create workflow
	wf := engine.Workflow{Name: "Original Name"}
	body, _ := json.Marshal(wf)
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var created engine.Workflow
	json.NewDecoder(rec.Body).Decode(&created)

	// Update workflow
	created.Name = "Updated Name"
	updateBody, _ := json.Marshal(created)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/workflows/"+created.ID, bytes.NewReader(updateBody))
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, updateRec.Code)
	}

	// Verify update
	getReq := httptest.NewRequest(http.MethodGet, "/api/workflows/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)

	var updated engine.Workflow
	json.NewDecoder(getRec.Body).Decode(&updated)

	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %q", updated.Name)
	}
}
