package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

// TriggerHandler handles trigger CRUD operations
type TriggerHandler struct {
	Store          storage.Storage
	TriggerManager *engine.TriggerManager
}

// NewTriggerHandler creates a new trigger handler
func NewTriggerHandler(store storage.Storage, manager *engine.TriggerManager) *TriggerHandler {
	return &TriggerHandler{
		Store:          store,
		TriggerManager: manager,
	}
}

// CreateTriggerRequest represents the request body for creating a trigger
type CreateTriggerRequest struct {
	WorkflowID string                 `json:"workflow_id"`
	Type       string                 `json:"type"` // cron, interval, webhook, typescript
	Config     map[string]interface{} `json:"config"`
	Enabled    bool                   `json:"enabled"`
	FilePath   string                 `json:"file_path"` // Path to the TypeScript trigger file
}

// Create handles POST /api/triggers
func (h *TriggerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.WorkflowID == "" {
		http.Error(w, "workflow_id is required", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}

	if req.Type != string(engine.TriggerTypeCron) && req.Type != string(engine.TriggerTypeInterval) &&
		req.Type != string(engine.TriggerTypeWebhook) && req.Type != string(engine.TriggerTypeTS) {
		http.Error(w, "type must be cron, interval, webhook, or typescript", http.StatusBadRequest)
		return
	}
	if req.Type == string(engine.TriggerTypeTS) && req.FilePath == "" {
		http.Error(w, "file_path is required for typescript triggers", http.StatusBadRequest)
		return
	}

	// Verify workflow exists
	_, err := h.Store.GetWorkflow(r.Context(), req.WorkflowID)
	if err != nil {
		http.Error(w, "Workflow not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Encode config as JSON
	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		http.Error(w, "Failed to encode config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create trigger
	trigger := &storage.Trigger{
		ID:         fmt.Sprintf("trigger_%d", time.Now().UnixNano()),
		WorkflowID: req.WorkflowID,
		Type:       req.Type,
		Config:     configBytes,
		Enabled:    req.Enabled,
		FilePath:   req.FilePath, // Assign FilePath
	}

	if err := h.Store.CreateTrigger(r.Context(), trigger); err != nil {
		http.Error(w, "Failed to create trigger: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If enabled, register with TriggerManager
	if trigger.Enabled {
		if err := h.registerTrigger(trigger); err != nil {

			fmt.Printf("Warning: failed to register trigger: %v\n", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(trigger)
}

// Get handles GET /api/triggers/{id}
func (h *TriggerHandler) Get(w http.ResponseWriter, r *http.Request) {
	triggerID := r.PathValue("id")
	if triggerID == "" {
		http.Error(w, "Missing trigger ID", http.StatusBadRequest)
		return
	}

	trigger, err := h.Store.GetTrigger(r.Context(), triggerID)
	if err != nil {
		http.Error(w, "Trigger not found: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trigger)
}

// List handles GET /api/triggers?workflow_id={id}
func (h *TriggerHandler) List(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")

	var triggers []*storage.Trigger
	var err error

	if workflowID != "" {
		triggers, err = h.Store.ListTriggers(r.Context(), workflowID)
	} else {
		triggers, err = h.Store.ListAllTriggers(r.Context())
	}

	if err != nil {
		http.Error(w, "Failed to list triggers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(triggers)
}

// Update handles PUT /api/triggers/{id}
func (h *TriggerHandler) Update(w http.ResponseWriter, r *http.Request) {
	triggerID := r.PathValue("id")
	if triggerID == "" {
		http.Error(w, "Missing trigger ID", http.StatusBadRequest)
		return
	}

	var req CreateTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate type
	if req.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}

	if req.Type != string(engine.TriggerTypeCron) && req.Type != string(engine.TriggerTypeInterval) &&
		req.Type != string(engine.TriggerTypeWebhook) && req.Type != string(engine.TriggerTypeTS) {
		http.Error(w, "type must be cron, interval, webhook, or typescript", http.StatusBadRequest)
		return
	}
	if req.Type == string(engine.TriggerTypeTS) && req.FilePath == "" {
		http.Error(w, "file_path is required for typescript triggers", http.StatusBadRequest)
		return
	}

	// Get existing trigger
	existing, err := h.Store.GetTrigger(r.Context(), triggerID)
	if err != nil {
		http.Error(w, "Trigger not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Encode config
	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		http.Error(w, "Failed to encode config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update trigger
	existing.WorkflowID = req.WorkflowID
	existing.Type = req.Type
	existing.Config = configBytes
	existing.FilePath = req.FilePath // Assign FilePath
	wasEnabled := existing.Enabled
	existing.Enabled = req.Enabled

	if err := h.Store.UpdateTrigger(r.Context(), existing); err != nil {
		http.Error(w, "Failed to update trigger: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle trigger manager updates
	if wasEnabled && !existing.Enabled {

		h.TriggerManager.Unregister(triggerID)
	} else if !wasEnabled && existing.Enabled {

		if err := h.registerTrigger(existing); err != nil {
			fmt.Printf("Warning: failed to register trigger: %v\n", err)
		}
	} else if existing.Enabled {

		h.TriggerManager.Unregister(triggerID)
		if err := h.registerTrigger(existing); err != nil {
			fmt.Printf("Warning: failed to register trigger: %v\n", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

// Delete handles DELETE /api/triggers/{id}
func (h *TriggerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	triggerID := r.PathValue("id")
	if triggerID == "" {
		http.Error(w, "Missing trigger ID", http.StatusBadRequest)
		return
	}

	// Unregister from TriggerManager first
	h.TriggerManager.Unregister(triggerID)

	// Delete from database
	if err := h.Store.DeleteTrigger(r.Context(), triggerID); err != nil {
		http.Error(w, "Failed to delete trigger: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListExecutions handles GET /api/triggers/{id}/executions
func (h *TriggerHandler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	triggerID := r.PathValue("id")
	if triggerID == "" {
		http.Error(w, "Missing trigger ID", http.StatusBadRequest)
		return
	}

	executions, err := h.Store.ListTriggerExecutions(r.Context(), triggerID, 100)
	if err != nil {
		http.Error(w, "Failed to list executions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

// registerTrigger creates and registers a trigger runner with the TriggerManager
func (h *TriggerHandler) registerTrigger(trigger *storage.Trigger) error {

	var config map[string]interface{}
	if err := json.Unmarshal(trigger.Config, &config); err != nil {
		return fmt.Errorf("failed to parse trigger config: %w", err)
	}

	var runner engine.TriggerRunner

	switch engine.TriggerType(trigger.Type) {
	case engine.TriggerTypeCron:
		schedule, ok := config["schedule"].(string)
		if !ok {
			return fmt.Errorf("cron trigger requires 'schedule' field")
		}
		runner = engine.NewCronTrigger(trigger.ID, trigger.WorkflowID, schedule, h.TriggerManager)

	case engine.TriggerTypeInterval:
		intervalSec, ok := config["interval"].(float64)
		if !ok {
			return fmt.Errorf("interval trigger requires 'interval' field (seconds)")
		}
		interval := time.Duration(intervalSec) * time.Second
		runner = engine.NewIntervalTrigger(trigger.ID, trigger.WorkflowID, interval, h.TriggerManager)

	case engine.TriggerTypeWebhook:
		runner = engine.NewWebhookTrigger(trigger.ID, trigger.WorkflowID, h.TriggerManager)

	case engine.TriggerTypeTS: // Handle TypeScript triggers
		if trigger.FilePath == "" {
			return fmt.Errorf("typescript trigger requires 'file_path'")
		}
		runner = engine.NewTSTriggerRunner(trigger.ID, trigger.WorkflowID, trigger.FilePath, config, h.TriggerManager)

	default:
		return fmt.Errorf("unsupported trigger type: %s", trigger.Type)
	}

	return h.TriggerManager.Register(runner)
}

// HandleWebhook handles POST /api/webhooks/{id}
func (h *TriggerHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	triggerID := r.PathValue("id")
	if triggerID == "" {
		http.Error(w, "Missing trigger ID", http.StatusBadRequest)
		return
	}

	// Get trigger from manager to access its runner instance
	triggerRunner, exists := h.TriggerManager.GetTrigger(triggerID)
	if !exists {
		http.Error(w, "Trigger not found", http.StatusNotFound)
		return
	}

	// Get trigger details from storage (for enabled status and type validation)
	triggerFromStore, err := h.Store.GetTrigger(r.Context(), triggerID)
	if err != nil {
		http.Error(w, "Trigger not found: "+err.Error(), http.StatusNotFound)
		return
	}

	if !triggerFromStore.Enabled {
		http.Error(w, "Trigger is disabled", http.StatusForbidden)
		return
	}

	// Ensure it's a webhook trigger (either Go-native or TS-based webhook)
	if triggerFromStore.Type != string(engine.TriggerTypeWebhook) && triggerFromStore.Type != string(engine.TriggerTypeTS) {
		http.Error(w, "Trigger is not a webhook type", http.StatusBadRequest)
		return
	}

	// Read body
	var body interface{}
	if r.Body != nil {
		defer r.Body.Close()

		json.NewDecoder(r.Body).Decode(&body)
	}

	// Construct payload
	payload := map[string]interface{}{
		"headers": r.Header,
		"method":  r.Method,
		"query":   r.URL.Query(),
		"body":    body,
	}

	// Check if it's a TypeScript trigger runner and invoke it directly
	if tsRunner, ok := triggerRunner.(*engine.TSTriggerRunner); ok {
		if err := tsRunner.Invoke(r.Context(), payload); err != nil {
			http.Error(w, "Failed to invoke TS webhook trigger: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {

		if err := h.TriggerManager.Fire(r.Context(), triggerID, payload); err != nil {
			http.Error(w, "Failed to fire Go-native webhook trigger: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
