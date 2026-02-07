package engine

import (
	"sync"
	"testing"
)

func TestStateManager_ConcurrentSetResult(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	sm := NewStateManager(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sm.SetResult("node-1", map[string]interface{}{"value": id})
		}(i)
	}

	wg.Wait()

	result := sm.GetResult("node-1")
	if result == nil {
		t.Error("expected result to be set")
	}
}

func TestStateManager_ConcurrentGetResult(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	sm := NewStateManager(ctx)

	sm.SetResult("node-1", map[string]interface{}{"data": "test"})

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := sm.GetResult("node-1")
			if result == nil {
				errors <- nil
			}
		}()
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		t.Error("concurrent reads should not return nil")
	}
}

func TestStateManager_PrepareInputWithVariables(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	ctx.SetVar("baseUrl", "https://api.example.com")

	sm := NewStateManager(ctx)

	node := &Node{
		ID:   "http-1",
		Type: NodeTypeHTTPRequest,
		Config: map[string]interface{}{
			"url": "${baseUrl}/users",
		},
	}

	input, err := sm.PrepareInput(node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, ok := input["config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected config map")
	}

	if config["url"] != "https://api.example.com/users" {
		t.Errorf("expected variable to be resolved, got: %v", config["url"])
	}
}

func TestStateManager_PrepareInputInvalidVariable(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	sm := NewStateManager(ctx)

	node := &Node{
		ID:   "http-1",
		Type: NodeTypeHTTPRequest,
		Config: map[string]interface{}{
			"url": "${undefinedVar}",
		},
	}

	_, err := sm.PrepareInput(node)
	if err == nil {
		t.Error("expected error for undefined variable")
	}
}

func TestStateManager_IsolationBetweenExecutions(t *testing.T) {
	ctx1 := NewExecutionContext("wf-1")
	ctx1.SetVar("env", "production")
	sm1 := NewStateManager(ctx1)
	sm1.SetResult("node-1", "result-1")

	ctx2 := NewExecutionContext("wf-1")
	ctx2.SetVar("env", "development")
	sm2 := NewStateManager(ctx2)
	sm2.SetResult("node-1", "result-2")

	if sm1.GetResult("node-1") == sm2.GetResult("node-1") {
		t.Error("expected isolated state between executions")
	}

	if ctx1.GetVar("env") == ctx2.GetVar("env") {
		t.Error("expected isolated variables between executions")
	}
}

func TestStateManager_NestedVariableResolution(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	ctx.SetVar("protocol", "https")
	ctx.SetVar("domain", "api.example.com")

	sm := NewStateManager(ctx)

	node := &Node{
		ID:   "http-1",
		Type: NodeTypeHTTPRequest,
		Config: map[string]interface{}{
			"url":    "${protocol}://${domain}/endpoint",
			"method": "GET",
		},
	}

	input, err := sm.PrepareInput(node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := input["config"].(map[string]interface{})
	expectedURL := "https://api.example.com/endpoint"

	if config["url"] != expectedURL {
		t.Errorf("expected %s, got %v", expectedURL, config["url"])
	}
}

func TestStateManager_GetResultNonExistent(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	sm := NewStateManager(ctx)

	result := sm.GetResult("nonexistent-node")

	if result != nil {
		t.Errorf("expected nil for nonexistent node, got %v", result)
	}
}

func TestStateManager_OverwriteResult(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	sm := NewStateManager(ctx)

	sm.SetResult("node-1", "first")
	sm.SetResult("node-1", "second")

	result := sm.GetResult("node-1")
	if result != "second" {
		t.Errorf("expected second value to overwrite first, got: %v", result)
	}
}

func TestStateManager_ConcurrentDifferentNodes(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	sm := NewStateManager(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			nodeID := string(rune('A' + id))
			sm.SetResult(nodeID, map[string]interface{}{"value": id})
		}(i)
	}

	wg.Wait()

	for i := 0; i < 10; i++ {
		nodeID := string(rune('A' + i))
		result := sm.GetResult(nodeID)
		if result == nil {
			t.Errorf("expected result for node %s", nodeID)
		}
	}
}

func TestStateManager_ComplexDataTypes(t *testing.T) {
	ctx := NewExecutionContext("wf-1")
	sm := NewStateManager(ctx)

	complexData := map[string]interface{}{
		"users": []map[string]interface{}{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
		"metadata": map[string]interface{}{
			"count": 2,
			"page":  1,
		},
	}

	sm.SetResult("node-1", complexData)

	result := sm.GetResult("node-1")
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	users, ok := resultMap["users"].([]map[string]interface{})
	if !ok || len(users) != 2 {
		t.Error("expected users array with 2 items")
	}
}
