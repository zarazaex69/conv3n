package engine_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zarazaex69/conv3n/internal/engine"
)

// TestBunRunner_ExecuteBlock_HTTPRequest verifies HTTP request block execution
func TestBunRunner_ExecuteBlock_HTTPRequest(t *testing.T) {

	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	cwd, _ := os.Getwd()
	blocksDir := filepath.Join(cwd, "../../pkg/blocks")
	if _, err := os.Stat(blocksDir); os.IsNotExist(err) {
		blocksDir = filepath.Join(cwd, "pkg/blocks")
	}

	runner := engine.NewBunRunner(blocksDir)

	// Setup mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success", "origin": "127.0.0.1"}`))
	}))
	defer ts.Close()

	// Create a simple HTTP request block
	block := engine.Block{
		ID:   "test-http",
		Type: engine.BlockTypeHTTPRequest,
		Config: map[string]interface{}{
			"url":    ts.URL,
			"method": "GET",
		},
	}

	input := map[string]interface{}{
		"config": block.Config,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runner.ExecuteBlock(ctx, block, input)
	if err != nil {
		t.Fatalf("ExecuteBlock failed: %v", err)
	}

	resMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	// Verify HTTP response structure

	dataMap, ok := resMap["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map in result, got %T", resMap["data"])
	}

	if dataMap["status"] == nil {
		t.Error("expected status field in result data")
	}
}

// TestBunRunner_ExecuteBlock_CustomCode verifies custom code block execution
func TestBunRunner_ExecuteBlock_CustomCode(t *testing.T) {

	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	cwd, _ := os.Getwd()
	blocksDir := filepath.Join(cwd, "../../pkg/blocks")
	if _, err := os.Stat(blocksDir); os.IsNotExist(err) {
		blocksDir = filepath.Join(cwd, "pkg/blocks")
	}

	runner := engine.NewBunRunner(blocksDir)

	// Create a custom code block
	block := engine.Block{
		ID:   "test-code",
		Type: engine.BlockTypeCustomCode,
		Config: map[string]interface{}{
			"code": "export default async (input) => { return { result: 42 }; }",
		},
	}

	input := map[string]interface{}{
		"config": block.Config,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := runner.ExecuteBlock(ctx, block, input)
	if err != nil {
		t.Fatalf("ExecuteBlock failed: %v", err)
	}

	resMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	// Verify custom code execution

	dataMap, ok := resMap["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map in result, got %T", resMap["data"])
	}

	if dataMap["success"] != true {
		t.Errorf("expected success=true, got %v", dataMap["success"])
	}
}

// TestBunRunner_ExecuteBlock_UnknownType verifies error handling for unknown block types
func TestBunRunner_ExecuteBlock_UnknownType(t *testing.T) {
	runner := engine.NewBunRunner("/tmp")

	block := engine.Block{
		ID:   "test-unknown",
		Type: "unknown/type",
		Config: map[string]interface{}{
			"test": "data",
		},
	}

	input := map[string]interface{}{
		"config": block.Config,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := runner.ExecuteBlock(ctx, block, input)
	if err == nil {
		t.Error("expected error for unknown block type, got nil")
	}
}

// TestBunRunner_Execute_ContextCancellation verifies context cancellation handling
func TestBunRunner_Execute_ContextCancellation(t *testing.T) {

	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not found in PATH, skipping test")
	}

	cwd, _ := os.Getwd()
	scriptPath := filepath.Join(cwd, "../../pkg/bunock/runner.ts")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = filepath.Join(cwd, "pkg/bunock/runner.ts")
	}

	runner := engine.NewBunRunner(filepath.Dir(scriptPath))

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := map[string]interface{}{"test": "data"}

	_, err := runner.Execute(ctx, scriptPath, input)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}
