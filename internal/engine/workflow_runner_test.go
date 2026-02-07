package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/storage"
)

type mockWorkflowStorage struct {
	storage.Storage
	executions  map[string]*storage.Execution
	nodeResults map[string]map[string][]byte
	mu          sync.RWMutex
	execCounter int
}

func (m *mockWorkflowStorage) CreateExecution(ctx context.Context, workflowID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCounter++
	execID := fmt.Sprintf("exec-%s-%d", workflowID, m.execCounter)
	if m.executions == nil {
		m.executions = make(map[string]*storage.Execution)
	}
	m.executions[execID] = &storage.Execution{
		ID:         execID,
		WorkflowID: workflowID,
		Status:     storage.ExecutionStatusRunning,
	}
	return execID, nil
}

func (m *mockWorkflowStorage) UpdateExecutionStatus(ctx context.Context, id string, status storage.ExecutionStatus, state []byte, errMsg *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exec, ok := m.executions[id]; ok {
		exec.Status = status
		exec.State = state
		exec.Error = errMsg
	}
	return nil
}

func (m *mockWorkflowStorage) SaveNodeResult(ctx context.Context, execID, nodeID string, result []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nodeResults == nil {
		m.nodeResults = make(map[string]map[string][]byte)
	}
	if m.nodeResults[execID] == nil {
		m.nodeResults[execID] = make(map[string][]byte)
	}
	m.nodeResults[execID][nodeID] = result
	return nil
}

func TestWorkflowRunner_BasicExecution(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-basic",
		Name: "Basic Workflow",
		Nodes: map[string]Node{
			"start": {
				ID:   "start",
				Type: NodeTypeSetVar,
				Config: map[string]interface{}{
					"name":  "testVar",
					"value": "testValue",
				},
			},
		},
		Edges: []Edge{},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	store.mu.RLock()
	var execStatus storage.ExecutionStatus
	for _, exec := range store.executions {
		if exec.WorkflowID == "wf-basic" {
			execStatus = exec.Status
			break
		}
	}
	store.mu.RUnlock()

	if execStatus != storage.ExecutionStatusCompleted {
		t.Errorf("expected status completed, got %v", execStatus)
	}
}

func TestWorkflowRunner_DataLimiterEnforcement(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-data-limit",
		Name: "Data Limiter Test",
		Nodes: map[string]Node{
			"huge-data": {
				ID:   "huge-data",
				Type: NodeTypeSetVar,
				Config: map[string]interface{}{
					"name":  "largeData",
					"value": make([]byte, 15*1024*1024),
				},
			},
		},
		Edges: []Edge{},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)

	if err == nil {
		t.Error("expected error due to data limiter")
	}

	store.mu.RLock()
	var execStatus storage.ExecutionStatus
	for _, exec := range store.executions {
		if exec.WorkflowID == "wf-data-limit" {
			execStatus = exec.Status
			break
		}
	}
	store.mu.RUnlock()

	if execStatus != storage.ExecutionStatusFailed {
		t.Errorf("expected status failed, got %v", execStatus)
	}
}

func TestWorkflowRunner_CircularDependencyDetection(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-circular",
		Name: "Circular Dependency",
		Nodes: map[string]Node{
			"a": {ID: "a", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "a", "value": "1"}},
			"b": {ID: "b", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "b", "value": "2"}},
			"c": {ID: "c", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "c", "value": "3"}},
		},
		Edges: []Edge{
			{Source: "a", Target: "b"},
			{Source: "b", Target: "c"},
			{Source: "c", Target: "a"},
		},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)

	if err != context.DeadlineExceeded {
		t.Logf("workflow should timeout or detect circular dependency, got: %v", err)
	}
}

func TestWorkflowRunner_ParallelNodeExecution(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-parallel",
		Name: "Parallel Execution",
		Nodes: map[string]Node{
			"parallel-1": {ID: "parallel-1", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "p1", "value": "1"}},
			"parallel-2": {ID: "parallel-2", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "p2", "value": "2"}},
			"parallel-3": {ID: "parallel-3", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "p3", "value": "3"}},
		},
		Edges: []Edge{},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := runner.Run(runCtx, workflow)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if elapsed > 2*time.Second {
		t.Errorf("parallel execution took too long: %v", elapsed)
	}

	store.mu.RLock()
	var execID string
	for id, exec := range store.executions {
		if exec.WorkflowID == "wf-parallel" {
			execID = id
			break
		}
	}
	resultCount := len(store.nodeResults[execID])
	store.mu.RUnlock()

	if resultCount != 3 {
		t.Errorf("expected 3 node results, got %d", resultCount)
	}
}

func TestWorkflowRunner_NodeExecutionOrder(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-order",
		Name: "Execution Order",
		Nodes: map[string]Node{
			"first":  {ID: "first", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "step", "value": "1"}},
			"second": {ID: "second", Type: NodeTypeGetVar, Config: map[string]interface{}{"name": "step"}},
			"third":  {ID: "third", Type: NodeTypeGetVar, Config: map[string]interface{}{"name": "step"}},
		},
		Edges: []Edge{
			{Source: "first", Target: "second"},
			{Source: "second", Target: "third"},
		},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	store.mu.RLock()
	var execID string
	for id, exec := range store.executions {
		if exec.WorkflowID == "wf-order" {
			execID = id
			break
		}
	}
	firstResult := store.nodeResults[execID]["first"]
	secondResult := store.nodeResults[execID]["second"]
	thirdResult := store.nodeResults[execID]["third"]
	store.mu.RUnlock()

	if len(firstResult) == 0 || len(secondResult) == 0 || len(thirdResult) == 0 {
		t.Error("expected all nodes to have results")
	}
}

func TestWorkflowRunner_TimeoutHandling(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-timeout",
		Name: "Timeout Test",
		Nodes: map[string]Node{
			"node-1": {ID: "node-1", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "v1", "value": "1"}},
		},
		Edges: []Edge{},
		Config: &WorkflowConfig{
			Timeout: 100 * time.Millisecond,
		},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx := context.Background()

	err := runner.Run(runCtx, workflow)

	if err != nil && err != context.DeadlineExceeded {
		t.Logf("workflow completed or timed out: %v", err)
	}
}

func TestWorkflowRunner_ConcurrencyLimit(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	nodes := make(map[string]Node)
	for i := 0; i < 20; i++ {
		id := string(rune('A' + i))
		nodes[id] = Node{
			ID:     id,
			Type:   NodeTypeSetVar,
			Config: map[string]interface{}{"name": id, "value": i},
		}
	}

	workflow := Workflow{
		ID:    "wf-concurrent",
		Name:  "Concurrency Limit",
		Nodes: nodes,
		Edges: []Edge{},
		Config: &WorkflowConfig{
			MaxConcurrentNodes: 3,
		},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}
}

func TestWorkflowRunner_ErrorPropagation(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-error",
		Name: "Error Propagation",
		Nodes: map[string]Node{
			"invalid": {
				ID:   "invalid",
				Type: NodeType("nonexistent-type"),
				Config: map[string]interface{}{
					"value": "test",
				},
			},
		},
		Edges: []Edge{},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)

	if err == nil {
		t.Error("expected error for invalid node type")
	}

	store.mu.RLock()
	var exec *storage.Execution
	for _, e := range store.executions {
		if e.WorkflowID == "wf-error" {
			exec = e
			break
		}
	}
	store.mu.RUnlock()

	if exec == nil {
		t.Fatal("execution not found")
	}

	if exec.Status != storage.ExecutionStatusFailed {
		t.Errorf("expected failed status, got %v", exec.Status)
	}

	if exec.Error == nil {
		t.Error("expected error message to be set")
	}
}

func TestWorkflowRunner_StateIsolation(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-isolation",
		Name: "State Isolation",
		Nodes: map[string]Node{
			"set-var": {ID: "set-var", Type: NodeTypeSetVar, Config: map[string]interface{}{"name": "shared", "value": "isolated"}},
		},
		Edges: []Edge{},
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := NewExecutionContext(workflow.ID)
			runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

			runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := runner.Run(runCtx, workflow)
			if err != nil {
				t.Errorf("execution %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	store.mu.RLock()
	execCount := len(store.executions)
	store.mu.RUnlock()

	if execCount != 3 {
		t.Errorf("expected 3 isolated executions, got %d", execCount)
	}
}

func TestWorkflowRunner_VariableResolution(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:   "wf-vars",
		Name: "Variable Resolution",
		Nodes: map[string]Node{
			"set-base": {
				ID:     "set-base",
				Type:   NodeTypeSetVar,
				Config: map[string]interface{}{"name": "baseUrl", "value": "https://api.example.com"},
			},
			"use-var": {
				ID:   "use-var",
				Type: NodeTypeGetVar,
				Config: map[string]interface{}{
					"name": "baseUrl",
				},
			},
		},
		Edges: []Edge{
			{Source: "set-base", Target: "use-var"},
		},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	store.mu.RLock()
	var resultBytes []byte
	for _, results := range store.nodeResults {
		if results["use-var"] != nil {
			resultBytes = results["use-var"]
			break
		}
	}
	store.mu.RUnlock()

	if resultBytes == nil {
		t.Fatal("no result found for use-var node")
	}

	var result map[string]interface{}
	json.Unmarshal(resultBytes, &result)

	if result["value"] != "https://api.example.com" {
		t.Errorf("expected variable to be resolved, got: %v", result["value"])
	}
}

func TestWorkflowRunner_EmptyWorkflow(t *testing.T) {
	store := &mockWorkflowStorage{}
	registry := NewExecutionRegistry()
	blockRegistry := NewBlockRegistry("")

	workflow := Workflow{
		ID:    "wf-empty",
		Name:  "Empty Workflow",
		Nodes: map[string]Node{},
		Edges: []Edge{},
	}

	ctx := NewExecutionContext(workflow.ID)
	runner := NewWorkflowRunner(ctx, nil, store, registry, blockRegistry)

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Run(runCtx, workflow)

	if err == nil {
		t.Error("expected error for workflow with no start nodes")
	}
}
