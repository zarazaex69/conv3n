package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/zarazaex69/conv3n/internal/storage"
)

// WorkflowRunner orchestrates the execution of a workflow.
// Deprecated: Use GraphRunner for new graph-based workflows.
type WorkflowRunner struct {
	bunRunner    *BunRunner
	stateManager *StateManager
	storage      storage.Storage
	registry     *ExecutionRegistry // Track active executions for cancellation
}

// NewWorkflowRunner creates a new runner for a specific execution context.
func NewWorkflowRunner(ctx *ExecutionContext, blocksDir string, store storage.Storage, registry *ExecutionRegistry) *WorkflowRunner {
	return &WorkflowRunner{
		bunRunner:    NewBunRunner(blocksDir),
		stateManager: NewStateManager(ctx),
		storage:      store,
		registry:     registry,
	}
}

func (wr *WorkflowRunner) LoadBlocks() error {
	return wr.bunRunner.LoadBlocks()
}
func (wr *WorkflowRunner) LoadBlocks() error {
	return wr.bunRunner.LoadBlocks()
}

// Run executes the workflow using the new graph-based engine.
// Automatically detects workflow format and uses appropriate execution strategy.
func (wr *WorkflowRunner) Run(ctx context.Context, workflow Workflow) error {
	log.Printf("Starting workflow: %s (%s)", workflow.Name, workflow.ID)

	// Use graph-based execution if workflow has nodes
	if len(workflow.Nodes) > 0 {
		return wr.runGraph(ctx, workflow)
	}

	// No nodes found - workflow might be empty or invalid
	return fmt.Errorf("workflow has no nodes to execute")
}

// runGraph executes the workflow using pointer-based graph traversal.
func (wr *WorkflowRunner) runGraph(ctx context.Context, workflow Workflow) error {
	// Create execution record
	execID, err := wr.storage.CreateExecution(ctx, workflow.ID)
	if err != nil {
		return fmt.Errorf("failed to create execution record: %w", err)
	}

	var finalStatus = storage.ExecutionStatusCompleted
	var finalError *string

	defer func() {
		stateBytes, _ := json.Marshal(wr.stateManager.ctx.Results)
		if err := wr.storage.UpdateExecutionStatus(ctx, execID, finalStatus, stateBytes, finalError); err != nil {
			log.Printf("Failed to update execution status: %v", err)
		}
	}()

	// Find start nodes (nodes with no incoming edges)
	startNodes := workflow.FindStartNodes()
	if len(startNodes) == 0 {
		return fmt.Errorf("no start nodes found in workflow")
	}

	// Execute from the first start node using pointer-based traversal
	startNodeID := startNodes[0]
	currentNodeID := startNodeID

	for currentNodeID != "" {
		// Check for context cancellation (kill switch)
		select {
		case <-ctx.Done():
			log.Printf("Execution cancelled: %v", ctx.Err())
			finalStatus = storage.ExecutionStatusCancelled
			msg := "Execution stopped by user"
			finalError = &msg
			return ctx.Err()
		default:
		}

		// Get the current node
		node := workflow.GetNode(currentNodeID)
		if node == nil {
			return fmt.Errorf("node not found: %s", currentNodeID)
		}

		log.Printf("Executing node: %s (%s)", node.ID, node.Type)

		// Prepare input by resolving variables
		resolvedConfig, err := ResolveVariables(node.Config, wr.stateManager.ctx)
		if err != nil {
			finalStatus = storage.ExecutionStatusFailed
			msg := err.Error()
			finalError = &msg
			return fmt.Errorf("failed to resolve variables for node %s: %w", node.ID, err)
		}

		input := map[string]interface{}{
			"config": resolvedConfig,
		}

		// Execute node via BunRunner
		rawResult, err := wr.bunRunner.ExecuteNode(ctx, node, input)
		if err != nil {
			finalStatus = storage.ExecutionStatusFailed
			msg := err.Error()
			finalError = &msg
			return fmt.Errorf("failed to execute node %s: %w", node.ID, err)
		}

		// Parse result to extract data and output port
		result := parseBlockResult(rawResult)

		// Process special actions (set_var, get_var, etc.)
		if err := wr.processNodeActions(node, result); err != nil {
			log.Printf("Warning: failed to process node actions: %v", err)
		}

		// Save result to context and storage
		wr.stateManager.SetResult(node.ID, result.Data)

		resBytes, _ := json.Marshal(result.Data)
		if err := wr.storage.SaveNodeResult(ctx, execID, node.ID, resBytes); err != nil {
			log.Printf("Warning: failed to save node result: %v", err)
		}

		log.Printf("Node %s completed, output port: %s", node.ID, result.Port)

		// Find the next node based on the output port
		currentNodeID = workflow.FindNextNode(node.ID, result.Port)
	}

	log.Printf("Workflow %s completed", workflow.ID)
	return nil
}

// parseBlockResult converts raw Bun output to BlockResult with port routing.
// IMPORTANT: We keep the full result structure (with "data" field) for variable resolution.
// Variables like {{ $node.block_1.data.value }} expect the "data" field to exist.
func parseBlockResult(raw interface{}) *BlockResult {
	result := &BlockResult{
		Data: raw, // Keep full structure for variable resolution
		Port: "default",
	}

	resMap, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}

	// Check if result has explicit port field
	if port, hasPort := resMap["port"]; hasPort {
		if portStr, ok := port.(string); ok {
			result.Port = portStr
		}
	}

	// For condition blocks, check data.result to determine port
	if data, hasData := resMap["data"]; hasData {
		if dataMap, ok := data.(map[string]interface{}); ok {
			if condResult, hasResult := dataMap["result"]; hasResult {
				if boolResult, ok := condResult.(bool); ok {
					if boolResult {
						result.Port = "true"
					} else {
						result.Port = "false"
					}
				}
			}
		}
	}

	return result
}

// processNodeActions handles special actions returned by blocks (e.g., set_var, get_var).
// This allows blocks to trigger side effects in the execution context.
func (wr *WorkflowRunner) processNodeActions(node *Node, result *BlockResult) error {
	// Check if result data contains an action field
	dataMap, ok := result.Data.(map[string]interface{})
	if !ok {
		return nil // No action to process
	}

	action, hasAction := dataMap["action"]
	if !hasAction {
		return nil // No action specified
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string, got %T", action)
	}

	switch actionStr {
	case "set_var":
		// Extract variable name and value
		name, hasName := dataMap["name"]
		if !hasName {
			return fmt.Errorf("set_var action requires 'name' field")
		}
		nameStr, ok := name.(string)
		if !ok {
			return fmt.Errorf("variable name must be a string, got %T", name)
		}

		value, hasValue := dataMap["value"]
		if !hasValue {
			return fmt.Errorf("set_var action requires 'value' field")
		}

		// Set the variable in execution context
		wr.stateManager.ctx.SetVar(nameStr, value)
		log.Printf("Set variable: %s = %v", nameStr, value)

	case "get_var":
		// get_var doesn't need special processing - the value was already resolved
		// by the variable resolver before the block executed
		log.Printf("Retrieved variable: %s", dataMap["name"])

	default:
		// Unknown action - log but don't fail
		log.Printf("Unknown action: %s", actionStr)
	}

	return nil
}

// RunLegacy executes a legacy linear workflow.
// Deprecated: Convert to graph format using LegacyWorkflow.ToGraphWorkflow().
func (wr *WorkflowRunner) RunLegacy(ctx context.Context, legacy LegacyWorkflow) error {
	// Convert to graph format and run
	workflow := legacy.ToGraphWorkflow()
	return wr.Run(ctx, *workflow)
}
