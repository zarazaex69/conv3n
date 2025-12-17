package engine

import (
	"testing"
)

// TestLegacyWorkflowConversion tests legacy to graph format conversion
func TestLegacyWorkflowConversion(t *testing.T) {
	legacy := LegacyWorkflow{
		ID:   "legacy-workflow",
		Name: "Legacy Test",
		Blocks: []Block{
			{ID: "start", Type: BlockTypeHTTPRequest},
			{ID: "process", Type: BlockTypeCustomCode},
			{ID: "end", Type: "std/transform"},
		},
		Connections: []Connection{
			{From: "start", To: "process"},
			{From: "process", To: "end"},
		},
	}

	graph := legacy.ToGraphWorkflow()

	// verify conversion preserves structure
	if len(graph.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(graph.Edges))
	}

	// verify edge mapping is correct
	expectedEdges := map[string]string{
		"start":   "process",
		"process": "end",
	}

	for _, edge := range graph.Edges {
		if expectedTarget, exists := expectedEdges[edge.Source]; !exists || expectedTarget != edge.Target {
			t.Errorf("Unexpected edge: %s -> %s", edge.Source, edge.Target)
		}
	}
}
