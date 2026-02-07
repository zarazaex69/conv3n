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
	store.disableShadowCheck = true
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

func TestVariableStore_ShadowingProtection(t *testing.T) {
	store := NewVariableStore()
	defer store.Close()

	workflowID := "wf1"
	executionID := "exec1"

	if err := store.Set(workflowID, executionID, "var", "global", ScopeGlobal, nil); err != nil {
		t.Fatalf("failed to set global variable: %v", err)
	}

	if err := store.Set(workflowID, executionID, "var", "workflow", ScopeWorkflow, nil); err == nil {
		t.Error("expected error when shadowing global variable from workflow scope")
	}

	if err := store.Set(workflowID, executionID, "var", "execution", ScopeExecution, nil); err == nil {
		t.Error("expected error when shadowing global variable from execution scope")
	}

	if err := store.Set(workflowID, executionID, "other", "workflow", ScopeWorkflow, nil); err != nil {
		t.Fatalf("failed to set workflow variable: %v", err)
	}

	if err := store.Set(workflowID, executionID, "other", "execution", ScopeExecution, nil); err == nil {
		t.Error("expected error when shadowing workflow variable from execution scope")
	}
}

func TestVariableStore_TypeSafety(t *testing.T) {
	store := NewVariableStore()
	defer store.Close()

	workflowID := "wf1"
	executionID := "exec1"

	if err := store.Set(workflowID, executionID, "numVar", 42, ScopeExecution, nil); err != nil {
		t.Fatalf("failed to set variable: %v", err)
	}

	val, err := store.GetWithTypeCheck(workflowID, executionID, "numVar", "number")
	if err != nil {
		t.Errorf("expected success with correct type, got error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}

	_, err = store.GetWithTypeCheck(workflowID, executionID, "numVar", "string")
	if err == nil {
		t.Error("expected type mismatch error")
	}
}
