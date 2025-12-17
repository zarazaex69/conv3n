package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zarazaex69/conv3n/internal/storage"
)

type ExecutionHandler struct {
	Store storage.Storage
}

func NewExecutionHandler(store storage.Storage) *ExecutionHandler {
	return &ExecutionHandler{Store: store}
}

type ExecutionResponse struct {
	ID          string                  `json:"id"`
	WorkflowID  string                  `json:"workflow_id"`
	Status      storage.ExecutionStatus `json:"status"`
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt *time.Time              `json:"completed_at,omitempty"`
	Error       *string                 `json:"error,omitempty"`
}

type ExecutionDetailResponse struct {
	ExecutionResponse
	State json.RawMessage `json:"state"`
}

func (h *ExecutionHandler) ListByWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	if workflowID == "" {
		http.Error(w, "Missing workflow ID", http.StatusBadRequest)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	execs, err := h.Store.ListExecutions(r.Context(), workflowID, limit)
	if err != nil {
		http.Error(w, "Failed to list executions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]ExecutionResponse, len(execs))
	for i, e := range execs {
		resp[i] = ExecutionResponse{
			ID:          e.ID,
			WorkflowID:  e.WorkflowID,
			Status:      e.Status,
			StartedAt:   e.StartedAt,
			CompletedAt: e.CompletedAt,
			Error:       e.Error,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ExecutionHandler) Get(w http.ResponseWriter, r *http.Request) {
	execID := r.PathValue("id")
	if execID == "" {
		http.Error(w, "Missing execution ID", http.StatusBadRequest)
		return
	}

	exec, err := h.Store.GetExecution(r.Context(), execID)
	if err != nil {
		http.Error(w, "Execution not found: "+err.Error(), http.StatusNotFound)
		return
	}

	resp := ExecutionDetailResponse{
		ExecutionResponse: ExecutionResponse{
			ID:          exec.ID,
			WorkflowID:  exec.WorkflowID,
			Status:      exec.Status,
			StartedAt:   exec.StartedAt,
			CompletedAt: exec.CompletedAt,
			Error:       exec.Error,
		},
		State: exec.State,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ExecutionHandler) GetNodeResult(w http.ResponseWriter, r *http.Request) {
	execID := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	if execID == "" || nodeID == "" {
		http.Error(w, "Missing execution or node ID", http.StatusBadRequest)
		return
	}

	result, err := h.Store.GetNodeResult(r.Context(), execID, nodeID)
	if err != nil {
		http.Error(w, "Node result not found: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}
