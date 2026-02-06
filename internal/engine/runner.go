package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

type BunRunner struct {
	RuntimePath string
	BlocksDir   string
	Registry    *BlockRegistry
}

func NewBunRunner(blocksDir string) *BunRunner {
	return &BunRunner{
		RuntimePath: "bun",
		BlocksDir:   blocksDir,
		Registry:    NewBlockRegistry(blocksDir),
	}
}

func (r *BunRunner) LoadBlocks() error {
	return r.Registry.LoadFromDirectory(r.BlocksDir)
}

// Execute runs the configured Bun script with the provided input payload.
// It writes the input to the subprocess's Stdin and reads the result from Stdout.
func (r *BunRunner) Execute(ctx context.Context, scriptPath string, input any) (any, error) {
	// Prepare the command: bun run <script>
	cmd := exec.CommandContext(ctx, r.RuntimePath, "run", scriptPath)

	// Setup pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start bun process: %w", err)
	}

	// Write input JSON to stdin
	// We run this in a goroutine to avoid deadlocks if the buffer fills up
	go func() {
		defer stdin.Close()
		if err := json.NewEncoder(stdin).Encode(input); err != nil {
			// In a real app, we might want to log this or handle it better
		}
	}()

	// Wait for the process to finish
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("bun execution failed: %v, stderr: %s", err, stderr.String())
	}

	// Log stderr for debugging (even on success)
	if stderr.Len() > 0 {
		fmt.Printf("[BunRunner stderr]: %s\n", stderr.String())
	}

	// Parse the output JSON
	var result any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse bun output: %w, raw output: %s, stderr: %s", err, stdout.String(), stderr.String())
	}

	return result, nil
}

// ExecuteBlock executes a specific block using the appropriate template.
// Deprecated: Use ExecuteNode for graph-based workflows.
func (r *BunRunner) ExecuteBlock(ctx context.Context, block Block, input any) (any, error) {
	scriptPath := r.getScriptPath(NodeType(block.Type))
	if scriptPath == "" {
		return nil, fmt.Errorf("unknown block type: %s", block.Type)
	}
	return r.Execute(ctx, scriptPath, input)
}

// ExecuteNode executes a node from the graph-based workflow.
// Returns raw result; caller is responsible for parsing port information.
func (r *BunRunner) ExecuteNode(ctx context.Context, node *Node, input any) (any, error) {
	scriptPath := r.getScriptPath(node.Type)
	if scriptPath == "" {
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}
	return r.Execute(ctx, scriptPath, input)
}

func (r *BunRunner) getScriptPath(nodeType NodeType) string {
	if nodeType == NodeTypeSetVar || nodeType == NodeTypeGetVar {
		return ""
	}

	manifest, ok := r.Registry.Get(nodeType)
	if !ok {
		return r.getFallbackPath(nodeType)
	}

	return manifest.ScriptPath
}

func (r *BunRunner) getFallbackPath(nodeType NodeType) string {
	switch nodeType {
	case NodeTypeHTTPRequest:
		return filepath.Join(r.BlocksDir, "std", "http_request.ts")
	case NodeTypeCustomCode:
		return filepath.Join(r.BlocksDir, "custom", "code.ts")
	case NodeTypeCondition:
		return filepath.Join(r.BlocksDir, "std", "condition.ts")
	case NodeTypeLoop:
		return filepath.Join(r.BlocksDir, "std", "loop.ts")
	case NodeTypeTransform:
		return filepath.Join(r.BlocksDir, "std", "transform.ts")
	case NodeTypeDelay:
		return filepath.Join(r.BlocksDir, "std", "delay.ts")
	case NodeTypeFile:
		return filepath.Join(r.BlocksDir, "std", "file.ts")
	case NodeTypeDatabase:
		return filepath.Join(r.BlocksDir, "std", "database.ts")
	case NodeTypeWebhook:
		return filepath.Join(r.BlocksDir, "std", "webhook.ts")
	default:
		return ""
	}
}
