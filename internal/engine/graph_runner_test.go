package engine_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

func TestGraphRunner(t *testing.T) {

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "graph_test.db")
	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Path to blocks directory (assuming test runs from internal/engine)

	absPath, _ := filepath.Abs("../../pkg/blocks")
	blocksDir := absPath

	t.Run("SimpleConditionWorkflow", func(t *testing.T) {

		// Start -> Condition (true) -> End

		wf := &engine.Workflow{
			ID:   "wf-graph-1",
			Name: "Graph Condition",
			Nodes: map[string]engine.Node{
				"start": {
					ID:   "start",
					Type: "std/condition",
					Config: map[string]interface{}{
						"expression": "1 == 1",
					},
				},
				"end_true": {
					ID:   "end_true",
					Type: "std/transform", // Just a dummy node to receive output
					Config: map[string]interface{}{
						"operations": []interface{}{
							map[string]interface{}{
								"type":   "pick",
								"fields": []string{"result"},
							},
						},
					},
				},
			},
			Edges: []engine.Edge{
				{
					ID:           "edge-1",
					Source:       "start",
					SourceHandle: "true",
					Target:       "end_true",
				},
			},
		}

		// Save workflow to storage (required for execution history)
		wfBytes, _ := json.Marshal(wf)
		store.CreateWorkflow(context.Background(), &storage.Workflow{
			ID:         wf.ID,
			Name:       wf.Name,
			Definition: wfBytes,
		})

		runner := engine.NewGraphRunner(wf, blocksDir, store)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := runner.Run(ctx)
		if err != nil {
			t.Fatalf("graph execution failed: %v", err)
		}

		// Verify results
		results := runner.GetResults()

		// Check start node result
		startRes, ok := results["start"].(map[string]interface{})
		if !ok {
			t.Fatal("expected start node result to be map")
		}
		if startRes["result"] != true {
			t.Errorf("expected condition result true, got %v", startRes["result"])
		}

		// Check end node execution (should have run)
		if _, ok := results["end_true"]; !ok {
			t.Error("expected end_true node to be executed")
		}
	})

	t.Run("VariableResolution", func(t *testing.T) {

		wf := &engine.Workflow{
			ID:   "wf-vars",
			Name: "Variable Workflow",
			Nodes: map[string]engine.Node{
				"start": {
					ID:   "start",
					Type: "std/condition",
					Config: map[string]interface{}{
						"expression": "'{{ $vars.foo }}' == 'bar'",
					},
				},
			},
			Edges: []engine.Edge{},
		}

		wfBytes, _ := json.Marshal(wf)
		store.CreateWorkflow(context.Background(), &storage.Workflow{
			ID:         wf.ID,
			Name:       wf.Name,
			Definition: wfBytes,
		})

		runner := engine.NewGraphRunner(wf, blocksDir, store)

		// Set variable
		runner.GetVariables()["foo"] = "bar"

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := runner.Run(ctx)
		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		results := runner.GetResults()
		startRes := results["start"].(map[string]interface{})
		if startRes["result"] != true {
			t.Errorf("expected condition to be true with variable, got %v", startRes["result"])
		}
	})
}
