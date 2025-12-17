package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarazaex69/conv3n/internal/engine"
	"github.com/zarazaex69/conv3n/internal/storage"
)

// MockTriggerManager allows us to intercept calls to Fire.
type MockTriggerManager struct {
	*engine.TriggerManager
	fireCalls chan FireCall
}

type FireCall struct {
	TriggerID string
	Payload   map[string]interface{}
}

func NewMockTriggerManager() *MockTriggerManager {
	// Using in-memory sqlite for tests
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		panic(fmt.Sprintf("failed to create mock storage: %v", err))
	}
	// The worker pool is necessary for the Fire method to work correctly.
	workerPool := engine.NewWorkerPool(2)

	tm := engine.NewTriggerManager(store, "", nil, workerPool)

	// In a real scenario, you'd stop the worker pool gracefully.
	// For tests, it's often sufficient to let it run until the test completes.

	return &MockTriggerManager{
		TriggerManager: tm,
		fireCalls:      make(chan FireCall, 10),
	}
}

// Fire is a mock implementation that intercepts calls to the real Fire method.
func (m *MockTriggerManager) Fire(ctx context.Context, triggerID string, payload map[string]interface{}) error {
	// Record the call for test assertions
	m.fireCalls <- FireCall{TriggerID: triggerID, Payload: payload}
	// Call the real Fire method to ensure the full flow is tested.
	return m.TriggerManager.Fire(ctx, triggerID, payload)
}

// setupTestTriggerFile creates a temporary TypeScript trigger file.
func setupTestTriggerFile(t *testing.T, content string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "trigger-test")
	require.NoError(t, err)
	filePath := filepath.Join(dir, "test_trigger.js") // Use .js for self-contained script
	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filePath
}

const basicTestTrigger = `
// This is a self-contained JS script that mimics the trigger IPC protocol for testing.
// It avoids any import/module resolution issues during 'go test'.
const stdin = process.stdin;
stdin.setEncoding('utf8');

// Function to send a message to the Go host
function sendMessage(message) {
  console.log(JSON.stringify(message));
}

stdin.on('data', (data) => {
  try {
    const msg = JSON.parse(data);

    if (msg.type === 'start') {
      // Received start, so send ready
      sendMessage({ type: 'status', status: 'ready' });
    } else if (msg.type === 'invoke') {
      // Received invoke, send an event back
      const requestId = Math.random().toString(36).substring(7);
      sendMessage({
        type: 'event',
        requestId: requestId,
        payload: { from: 'invoke', originalPayload: msg.payload }
      });
    } else if (msg.type === 'kill') {
      process.exit(0);
    }
  } catch (e) {
    // Ignore parse errors for this simple test script
  }
});
`

func TestTSTriggerRunner_StartAndStop(t *testing.T) {
	filePath := setupTestTriggerFile(t, basicTestTrigger)
	manager := NewMockTriggerManager()
	runner := engine.NewTSTriggerRunner("test-trigger-1", "wf-1", filePath, nil, manager.TriggerManager)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Start the runner
	err := runner.Start(ctx)
	require.NoError(t, err, "Runner should start without error")

	// Give it a moment to be fully ready
	time.Sleep(1 * time.Second)

	// Stop the runner
	err = runner.Stop()
	require.NoError(t, err, "Runner should stop without error")
}

func TestTSTriggerRunner_Invoke(t *testing.T) {
	filePath := setupTestTriggerFile(t, basicTestTrigger)
	manager := NewMockTriggerManager()
	runner := engine.NewTSTriggerRunner("test-trigger-invoke", "wf-1", filePath, nil, manager)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create dummy workflow and trigger in the store
	err := manager.Store.CreateWorkflow(ctx, &storage.Workflow{ID: "wf-1", Name: "Test Workflow", Definition: []byte("{}")})
	require.NoError(t, err)
	err = manager.Store.CreateTrigger(ctx, &storage.Trigger{
		ID:         "test-trigger-invoke",
		WorkflowID: "wf-1",
		Type:       "typescript",
		FilePath:   filePath,
		Enabled:    true,
		Config:     []byte("{}"), // Provide non-null config
	})
	require.NoError(t, err)

	// Register the runner (which also starts it)
	err = manager.Register(runner)
	require.NoError(t, err)

	// Invoke the trigger
	invokePayload := map[string]interface{}{"data": "hello from test"}
	err = runner.Invoke(ctx, invokePayload)
	require.NoError(t, err)

	// Check if Fire was called with the correct payload
	select {
	case call := <-manager.fireCalls:
		assert.Equal(t, "test-trigger-invoke", call.TriggerID)
		firedPayload := call.Payload
		assert.Equal(t, "invoke", firedPayload["from"])
		originalPayload, ok := firedPayload["originalPayload"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "hello from test", originalPayload["data"])
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for trigger to fire")
	}

	err = runner.Stop()
	require.NoError(t, err)
}

const eventFiringTrigger = `
// This is a self-contained JS script that fires an event on start.
const stdin = process.stdin;
stdin.setEncoding('utf8');

function sendMessage(message) {
  console.log(JSON.stringify(message));
}

stdin.on('data', (data) => {
  try {
    const msg = JSON.parse(data);
    if (msg.type === 'start') {
      // Send ready first
      sendMessage({ type: 'status', status: 'ready' });

      // Then fire an event
      const requestId = Math.random().toString(36).substring(7);
      sendMessage({
        type: 'event',
        requestId: requestId,
        payload: { from: 'onStart' }
      });

    } else if (msg.type === 'kill') {
      process.exit(0);
    }
  } catch (e) {
    // Ignore
  }
});
`

func TestTSTriggerRunner_ReceivesEvent(t *testing.T) {
	filePath := setupTestTriggerFile(t, eventFiringTrigger)
	manager := NewMockTriggerManager()
	runner := engine.NewTSTriggerRunner("test-trigger-event", "wf-1", filePath, nil, manager)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create dummy workflow and trigger in the store
	err := manager.Store.CreateWorkflow(ctx, &storage.Workflow{ID: "wf-1", Name: "Test Workflow", Definition: []byte("{}")})
	require.NoError(t, err)
	err = manager.Store.CreateTrigger(ctx, &storage.Trigger{
		ID:         "test-trigger-event",
		WorkflowID: "wf-1",
		Type:       "typescript",
		FilePath:   filePath,
		Enabled:    true,
		Config:     []byte("{}"), // Provide non-null config
	})
	require.NoError(t, err)

	// Register the runner (which also starts it)
	err = manager.Register(runner)
	require.NoError(t, err)

	// Check if Fire was called from the onStart event
	select {
	case call := <-manager.fireCalls:
		assert.Equal(t, "test-trigger-event", call.TriggerID)
		assert.Equal(t, "onStart", call.Payload["from"])
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for trigger to fire")
	}

	err = runner.Stop()
	require.NoError(t, err)
}

func TestTSTriggerRunner_Start_NonExistentFile(t *testing.T) {
	manager := NewMockTriggerManager()
	runner := engine.NewTSTriggerRunner("test-trigger-bad", "wf-1", "/non/existent/file.ts", nil, manager.TriggerManager)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runner.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TypeScript trigger file not found")
}
