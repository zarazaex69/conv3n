package engine

import (
	"context"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/storage"
)

func TestGraphRunnerEvents(t *testing.T) {
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	workflow := &Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Nodes: map[string]Node{
			"node1": {
				ID:   "node1",
				Type: NodeTypeDelay,
				Config: map[string]interface{}{
					"duration": 10,
					"unit":     "ms",
				},
			},
			"node2": {
				ID:   "node2",
				Type: NodeTypeDelay,
				Config: map[string]interface{}{
					"duration": 10,
					"unit":     "ms",
				},
			},
		},
		Edges: []Edge{
			{ID: "e1", Source: "node1", Target: "node2"},
		},
	}

	runner := NewGraphRunner(workflow, "../../pkg/blocks", store)

	var events []Event
	runner.SetEventListener(EventListenerFunc(func(e Event) {
		events = append(events, e)
	}))

	ctx := context.Background()
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("no events emitted")
	}

	eventTypes := make(map[EventType]int)
	for _, e := range events {
		eventTypes[e.Type]++

		if e.ExecutionID == "" {
			t.Errorf("event %s has empty ExecutionID", e.Type)
		}
		if e.WorkflowID != workflow.ID {
			t.Errorf("event %s has wrong WorkflowID: got %s, want %s", e.Type, e.WorkflowID, workflow.ID)
		}
		if e.Timestamp.IsZero() {
			t.Errorf("event %s has zero Timestamp", e.Type)
		}
	}

	if eventTypes[EventTypeExecutionStart] != 1 {
		t.Errorf("expected 1 execution.start event, got %d", eventTypes[EventTypeExecutionStart])
	}
	if eventTypes[EventTypeExecutionComplete] != 1 {
		t.Errorf("expected 1 execution.complete event, got %d", eventTypes[EventTypeExecutionComplete])
	}
	if eventTypes[EventTypeNodeStart] != 2 {
		t.Errorf("expected 2 node.start events, got %d", eventTypes[EventTypeNodeStart])
	}
	if eventTypes[EventTypeNodeComplete] != 2 {
		t.Errorf("expected 2 node.complete events, got %d", eventTypes[EventTypeNodeComplete])
	}

	firstEvent := events[0]
	if firstEvent.Type != EventTypeExecutionStart {
		t.Errorf("first event should be execution.start, got %s", firstEvent.Type)
	}

	lastEvent := events[len(events)-1]
	if lastEvent.Type != EventTypeExecutionComplete {
		t.Errorf("last event should be execution.complete, got %s", lastEvent.Type)
	}

	if data, ok := lastEvent.Data.(ExecutionCompleteData); ok {
		if data.Duration <= 0 {
			t.Error("execution duration should be positive")
		}
		if data.Results == nil {
			t.Error("execution results should not be nil")
		}
	} else {
		t.Error("execution.complete event should have ExecutionCompleteData")
	}
}

func TestGraphRunnerEventsOnError(t *testing.T) {
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	workflow := &Workflow{
		ID:   "test-workflow-error",
		Name: "Test Workflow Error",
		Nodes: map[string]Node{
			"node1": {
				ID:   "node1",
				Type: NodeTypeTransform,
				Config: map[string]interface{}{
					"expression": "invalid javascript syntax {{{",
				},
			},
		},
		Edges: []Edge{},
	}

	runner := NewGraphRunner(workflow, "../../pkg/blocks", store)

	var events []Event
	runner.SetEventListener(EventListenerFunc(func(e Event) {
		events = append(events, e)
	}))

	ctx := context.Background()
	if err := runner.Run(ctx); err == nil {
		t.Fatal("expected workflow to fail")
	}

	eventTypes := make(map[EventType]int)
	for _, e := range events {
		eventTypes[e.Type]++
	}

	if eventTypes[EventTypeExecutionStart] != 1 {
		t.Errorf("expected 1 execution.start event, got %d", eventTypes[EventTypeExecutionStart])
	}
	if eventTypes[EventTypeExecutionError] != 1 {
		t.Errorf("expected 1 execution.error event, got %d", eventTypes[EventTypeExecutionError])
	}
	if eventTypes[EventTypeNodeError] < 1 {
		t.Errorf("expected at least 1 node.error event, got %d", eventTypes[EventTypeNodeError])
	}

	lastEvent := events[len(events)-1]
	if lastEvent.Type != EventTypeExecutionError {
		t.Errorf("last event should be execution.error, got %s", lastEvent.Type)
	}
	if lastEvent.Error == nil {
		t.Error("execution.error event should have Error field set")
	}
}

func TestGraphRunnerEventsTimeout(t *testing.T) {
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	workflow := &Workflow{
		ID:   "test-workflow-timeout",
		Name: "Test Workflow Timeout",
		Nodes: map[string]Node{
			"node1": {
				ID:   "node1",
				Type: NodeTypeDelay,
				Config: map[string]interface{}{
					"duration": 10,
					"unit":     "ms",
				},
			},
		},
		Edges: []Edge{},
	}

	runner := NewGraphRunner(workflow, "../../pkg/blocks", store)

	var allEvents []Event
	runner.SetEventListener(EventListenerFunc(func(e Event) {
		allEvents = append(allEvents, e)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	runner.Run(ctx)

	eventTypes := make(map[EventType]int)
	for _, e := range allEvents {
		eventTypes[e.Type]++
	}

	if eventTypes[EventTypeExecutionStart] != 1 {
		t.Errorf("expected 1 execution.start event, got %d", eventTypes[EventTypeExecutionStart])
	}

	hasErrorEvent := eventTypes[EventTypeNodeError] > 0 || eventTypes[EventTypeExecutionError] > 0
	if !hasErrorEvent {
		t.Error("expected error event when context is cancelled")
	}
}

func TestEventListenerFunc(t *testing.T) {
	called := false
	listener := EventListenerFunc(func(e Event) {
		called = true
		if e.Type != EventTypeExecutionStart {
			t.Errorf("expected execution.start, got %s", e.Type)
		}
	})

	listener.OnEvent(Event{
		Type:      EventTypeExecutionStart,
		Timestamp: time.Now(),
	})

	if !called {
		t.Error("listener function was not called")
	}
}
