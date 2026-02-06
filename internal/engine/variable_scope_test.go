package engine

import (
	"testing"
	"time"
)

func TestVariableStore_ScopeHierarchy(t *testing.T) {
	store := NewVariableStore()
	defer store.Close()

	workflowID := "wf1"
	executionID := "exec1"

	store.Set(workflowID, executionID, "global_var", "global_value", ScopeGlobal, nil)
	store.Set(workflowID, executionID, "workflow_var", "workflow_value", ScopeWorkflow, nil)
	store.Set(workflowID, executionID, "exec_var", "exec_value", ScopeExecution, nil)

	if val, ok := store.Get(workflowID, executionID, "exec_var"); !ok || val != "exec_value" {
		t.Errorf("expected exec_value, got %v", val)
	}

	if val, ok := store.Get(workflowID, executionID, "workflow_var"); !ok || val != "workflow_value" {
		t.Errorf("expected workflow_value, got %v", val)
	}

	if val, ok := store.Get(workflowID, executionID, "global_var"); !ok || val != "global_value" {
		t.Errorf("expected global_value, got %v", val)
	}
}

func TestVariableStore_TTL(t *testing.T) {
	store := NewVariableStore()
	defer store.Close()

	workflowID := "wf1"
	executionID := "exec1"

	ttl := 100 * time.Millisecond
	store.Set(workflowID, executionID, "temp_var", "temp_value", ScopeExecution, &ttl)

	if val, ok := store.Get(workflowID, executionID, "temp_var"); !ok || val != "temp_value" {
		t.Errorf("expected temp_value immediately, got %v", val)
	}

	time.Sleep(150 * time.Millisecond)

	if _, ok := store.Get(workflowID, executionID, "temp_var"); ok {
		t.Error("expected variable to expire after TTL")
	}
}

func TestVariableStore_ClearExecution(t *testing.T) {
	store := NewVariableStore()
	defer store.Close()

	workflowID := "wf1"
	executionID := "exec1"

	store.Set(workflowID, executionID, "exec_var", "value", ScopeExecution, nil)
	store.Set(workflowID, executionID, "workflow_var", "value", ScopeWorkflow, nil)

	store.ClearExecution(executionID)

	if _, ok := store.Get(workflowID, executionID, "exec_var"); ok {
		t.Error("execution variable should be cleared")
	}

	if _, ok := store.Get(workflowID, executionID, "workflow_var"); !ok {
		t.Error("workflow variable should persist")
	}
}

func TestVariableStore_ScopePriority(t *testing.T) {
	store := NewVariableStore()
	defer store.Close()

	workflowID := "wf1"
	executionID := "exec1"

	store.Set(workflowID, executionID, "var", "global", ScopeGlobal, nil)
	store.Set(workflowID, executionID, "var", "workflow", ScopeWorkflow, nil)
	store.Set(workflowID, executionID, "var", "execution", ScopeExecution, nil)

	if val, ok := store.Get(workflowID, executionID, "var"); !ok || val != "execution" {
		t.Errorf("expected execution scope to have priority, got %v", val)
	}

	store.Delete(workflowID, executionID, "var")

	if _, ok := store.Get(workflowID, executionID, "var"); ok {
		t.Error("all scopes should be deleted")
	}
}
