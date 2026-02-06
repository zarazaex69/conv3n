package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type BunRunner struct {
	RuntimePath    string
	BlocksDir      string
	Registry       *BlockRegistry
	DefaultTimeout time.Duration
	RateLimiter    *RateLimiter
}

func NewBunRunner(blocksDir string) *BunRunner {
	limiter := NewRateLimiter()
	limiter.SetLimit(NodeTypeHTTPRequest, 100, 1*time.Second)
	limiter.SetLimit(NodeTypeDatabase, 50, 1*time.Second)

	return &BunRunner{
		RuntimePath:    "bun",
		BlocksDir:      blocksDir,
		Registry:       NewBlockRegistry(blocksDir),
		DefaultTimeout: 30 * time.Second,
		RateLimiter:    limiter,
	}
}

func (r *BunRunner) LoadBlocks() error {
	return r.Registry.LoadFromDirectory(r.BlocksDir)
}

// Execute runs the configured Bun script with the provided input payload.
// It writes the input to the subprocess's Stdin and reads the result from Stdout.
func (r *BunRunner) Execute(ctx context.Context, scriptPath string, input any) (any, error) {
	execCtx, cancel := context.WithTimeout(ctx, r.DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, r.RuntimePath, "run", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start bun process: %w", err)
	}

	stdinErrChan := make(chan error, 1)
	go func() {
		defer stdin.Close()
		if encErr := json.NewEncoder(stdin).Encode(input); encErr != nil {
			stdinErrChan <- encErr
		}
		close(stdinErrChan)
	}()

	waitErr := cmd.Wait()

	if execCtx.Err() == context.DeadlineExceeded {
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil, fmt.Errorf("bun execution timeout after %v", r.DefaultTimeout)
	}

	if stdinErr := <-stdinErrChan; stdinErr != nil {
		return nil, fmt.Errorf("failed to write input to stdin: %w", stdinErr)
	}

	if waitErr != nil {
		return nil, fmt.Errorf("bun execution failed: %v, stderr: %s", waitErr, stderr.String())
	}

	if stderr.Len() > 0 {
		fmt.Printf("[BunRunner stderr]: %s\n", stderr.String())
	}

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
	if err := r.RateLimiter.Wait(ctx, node.Type); err != nil {
		return nil, fmt.Errorf("rate limit exceeded for %s: %w", node.Type, err)
	}

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
