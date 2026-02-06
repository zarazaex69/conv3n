package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

// WorkflowHandler handles HTTP requests for workflow management
type WorkflowHandler struct {
	Store storage.Storage
}

// NewWorkflowHandler creates a new WorkflowHandler
func NewWorkflowHandler(store storage.Storage) *WorkflowHandler {
	return &WorkflowHandler{Store: store}
}

// Create handles POST /api/workflows
func (h *WorkflowHandler) Create(w http.ResponseWriter, r *http.Request) {
	var wf engine.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if wf.ID == "" {

		wf.ID = fmt.Sprintf("wf_%d", time.Now().UnixNano())
	}

	// Marshal definition back to bytes to store
	defBytes, err := json.Marshal(wf)
	if err != nil {
		http.Error(w, "Failed to marshal definition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	storedWf := &storage.Workflow{
		ID:         wf.ID,
		Name:       wf.Name,
		Definition: defBytes,
	}

	if err := h.Store.CreateWorkflow(r.Context(), storedWf); err != nil {
		http.Error(w, "Failed to create workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(wf)
}

// Get handles GET /api/workflows/{id}
func (h *WorkflowHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	storedWf, err := h.Store.GetWorkflow(r.Context(), id)
	if err != nil {
		http.Error(w, "Workflow not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Return the raw definition (which is the engine.Workflow JSON)
	w.Header().Set("Content-Type", "application/json")
	w.Write(storedWf.Definition)
}

// Update handles PUT /api/workflows/{id}
func (h *WorkflowHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	var wf engine.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Ensure ID in body matches ID in path
	wf.ID = id

	defBytes, err := json.Marshal(wf)
	if err != nil {
		http.Error(w, "Failed to marshal definition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	storedWf := &storage.Workflow{
		ID:         id,
		Name:       wf.Name,
		Definition: defBytes,
	}

	if err := h.Store.UpdateWorkflow(r.Context(), storedWf); err != nil {
		http.Error(w, "Failed to update workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

// Delete handles DELETE /api/workflows/{id}
func (h *WorkflowHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	if err := h.Store.DeleteWorkflow(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// List handles GET /api/workflows
func (h *WorkflowHandler) List(w http.ResponseWriter, r *http.Request) {
	storedWfs, err := h.Store.ListWorkflows(r.Context())
	if err != nil {
		http.Error(w, "Failed to list workflows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type WorkflowListItem struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	list := make([]WorkflowListItem, len(storedWfs))
	for i, sw := range storedWfs {
		list[i] = WorkflowListItem{
			ID:        sw.ID,
			Name:      sw.Name,
			CreatedAt: sw.CreatedAt,
			UpdatedAt: sw.UpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
