package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/zarazaex69/conv3n/internal/storage"
)

// GraphRunner executes workflows using pointer-based graph traversal.

type GraphRunner struct {
	workflow      *Workflow
	bunRunner     *BunRunner
	ctx           *ExecutionContext
	storage       storage.Storage
	executionID   string
	lastNodeID    string
	eventListener EventListener
}

const defaultNodeTimeout = 30 * time.Second

type resumeState struct {
	Results       map[string]interface{} `json:"results"`
	Variables     map[string]interface{} `json:"variables"`
	CurrentNodeID string                 `json:"current_node_id"`
}

// NewGraphRunner creates a new graph-based workflow runner.
func NewGraphRunner(workflow *Workflow, blocksDir string, store storage.Storage) *GraphRunner {
	return &GraphRunner{
		workflow:      workflow,
		bunRunner:     NewBunRunner(blocksDir),
		ctx:           NewExecutionContext(workflow.ID),
		storage:       store,
		eventListener: nil,
	}
}

func (gr *GraphRunner) SetEventListener(listener EventListener) {
	gr.eventListener = listener
}

func (gr *GraphRunner) emitEvent(event Event) {
	if gr.eventListener != nil {
		gr.eventListener.OnEvent(event)
	}
}

func getNodeTimeout(node *Node) time.Duration {
	if node == nil {
		return defaultNodeTimeout
	}

	if node.Config == nil {
		return defaultNodeTimeout
	}

	if raw, ok := node.Config["timeout_ms"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				return time.Duration(v) * time.Millisecond
			}
		}
	}

	return defaultNodeTimeout
}

// Run executes the workflow starting from the first node without incoming edges.

func (gr *GraphRunner) Run(ctx context.Context) error {
	log.Printf("Starting graph workflow: %s (%s)", gr.workflow.Name, gr.workflow.ID)

	execID, err := gr.storage.CreateExecution(ctx, gr.workflow.ID)
	if err != nil {
		return fmt.Errorf("failed to create execution record: %w", err)
	}
	gr.executionID = execID
	gr.ctx.ExecutionID = execID

	startNodes := gr.workflow.FindStartNodes()
	if len(startNodes) == 0 {
		return fmt.Errorf("no start nodes found in workflow")
	}

	startTime := time.Now()

	gr.emitEvent(Event{
		Type:        EventTypeExecutionStart,
		Timestamp:   startTime,
		ExecutionID: execID,
		WorkflowID:  gr.workflow.ID,
		Data: ExecutionStartData{
			StartNodes: startNodes,
		},
	})

	var finalStatus = storage.ExecutionStatusCompleted
	var finalError *string

	defer func() {
		state := resumeState{
			Results:       gr.ctx.Results,
			Variables:     gr.ctx.Variables,
			CurrentNodeID: gr.lastNodeID,
		}
		stateBytes, _ := json.Marshal(state)
		if err := gr.storage.UpdateExecutionStatus(ctx, execID, finalStatus, stateBytes, finalError); err != nil {
			log.Printf("Failed to update execution status: %v", err)
		}

		if finalStatus == storage.ExecutionStatusCompleted {
			gr.emitEvent(Event{
				Type:        EventTypeExecutionComplete,
				Timestamp:   time.Now(),
				ExecutionID: execID,
				WorkflowID:  gr.workflow.ID,
				Data: ExecutionCompleteData{
					Duration: time.Since(startTime),
					Results:  gr.ctx.Results,
				},
			})
		} else if finalStatus == storage.ExecutionStatusFailed {
			var execErr error
			if finalError != nil {
				execErr = errors.New(*finalError)
			}
			gr.emitEvent(Event{
				Type:        EventTypeExecutionError,
				Timestamp:   time.Now(),
				ExecutionID: execID,
				WorkflowID:  gr.workflow.ID,
				Error:       execErr,
			})
		}
	}()

	startNodeID := startNodes[0]

	if err := gr.executeFromNode(ctx, startNodeID); err != nil {
		finalStatus = storage.ExecutionStatusFailed
		msg := err.Error()
		finalError = &msg
		return err
	}

	log.Printf("Graph workflow %s completed", gr.workflow.ID)
	return nil
}

// executeFromNode executes the workflow starting from the given node.

func (gr *GraphRunner) executeFromNode(ctx context.Context, startNodeID string) error {
	currentNodeID := startNodeID

	for currentNodeID != "" {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get the current node
		node := gr.workflow.GetNode(currentNodeID)
		if node == nil {
			return fmt.Errorf("node not found: %s", currentNodeID)
		}

		log.Printf("Executing node: %s (%s)", node.ID, node.Type)

		var (
			result *BlockResult
			port   string
		)

		cachedBytes, err := gr.storage.GetNodeResult(ctx, gr.executionID, node.ID)
		if err == nil {
			var cachedData interface{}
			if err := json.Unmarshal(cachedBytes, &cachedData); err != nil {
				log.Printf("Warning: failed to unmarshal cached node result for %s: %v", node.ID, err)
			} else {
				gr.ctx.SetResult(node.ID, cachedData)
				gr.lastNodeID = node.ID
				log.Printf("Node %s skipped execution, using cached result", node.ID)
				port = ""
			}
		}

		if port == "" && gr.ctx.GetResult(node.ID) == nil {
			res, execErr := gr.executeNode(ctx, node)
			if execErr != nil {
				return fmt.Errorf("failed to execute node %s: %w", node.ID, execErr)
			}
			result = res
			gr.ctx.SetResult(node.ID, result.Data)
			gr.lastNodeID = node.ID

			resBytes, _ := json.Marshal(result.Data)
			if err := gr.storage.SaveNodeResult(ctx, gr.executionID, node.ID, resBytes); err != nil {
				log.Printf("Warning: failed to save node result: %v", err)
			}

			port = result.Port
			log.Printf("Node %s completed, output port: %s", node.ID, port)
		}

		// Find the next node based on the output port
		currentNodeID = gr.workflow.FindNextNode(node.ID, port)
	}

	return nil
}

// executeNode executes a single node and returns the result with output port.
func (gr *GraphRunner) executeNode(ctx context.Context, node *Node) (*BlockResult, error) {
	nodeTimeout := getNodeTimeout(node)
	nodeCtx, cancel := context.WithTimeout(ctx, nodeTimeout)
	defer cancel()

	nodeStartTime := time.Now()

	gr.emitEvent(Event{
		Type:        EventTypeNodeStart,
		Timestamp:   nodeStartTime,
		ExecutionID: gr.executionID,
		WorkflowID:  gr.workflow.ID,
		NodeID:      node.ID,
		Data: NodeStartData{
			NodeType: node.Type,
			Config:   node.Config,
		},
	})

	resolvedConfig, err := ResolveVariables(node.Config, gr.ctx)
	if err != nil {
		gr.emitEvent(Event{
			Type:        EventTypeNodeError,
			Timestamp:   time.Now(),
			ExecutionID: gr.executionID,
			WorkflowID:  gr.workflow.ID,
			NodeID:      node.ID,
			Error:       fmt.Errorf("failed to resolve variables: %w", err),
		})
		return nil, fmt.Errorf("failed to resolve variables: %w", err)
	}

	input := map[string]interface{}{
		"config": resolvedConfig,
		"context": map[string]interface{}{
			"workflowId":  gr.ctx.WorkflowID,
			"executionId": gr.ctx.ExecutionID,
			"variables":   gr.ctx.Variables,
		},
	}

	rawResult, err := gr.bunRunner.ExecuteNode(nodeCtx, node, input)
	if err != nil {
		var nodeErr error
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(nodeCtx.Err(), context.DeadlineExceeded) {
			nodeErr = fmt.Errorf("node %s execution timed out after %s: %w", node.ID, nodeTimeout, err)
		} else if errors.Is(err, context.Canceled) || errors.Is(nodeCtx.Err(), context.Canceled) {
			nodeErr = fmt.Errorf("node %s execution canceled: %w", node.ID, err)
		} else {
			nodeErr = err
		}

		gr.emitEvent(Event{
			Type:        EventTypeNodeError,
			Timestamp:   time.Now(),
			ExecutionID: gr.executionID,
			WorkflowID:  gr.workflow.ID,
			NodeID:      node.ID,
			Error:       nodeErr,
		})
		return nil, nodeErr
	}

	result, err := gr.parseBlockResult(rawResult)
	if err != nil {
		gr.emitEvent(Event{
			Type:        EventTypeNodeError,
			Timestamp:   time.Now(),
			ExecutionID: gr.executionID,
			WorkflowID:  gr.workflow.ID,
			NodeID:      node.ID,
			Error:       err,
		})
		return nil, err
	}

	gr.emitEvent(Event{
		Type:        EventTypeNodeComplete,
		Timestamp:   time.Now(),
		ExecutionID: gr.executionID,
		WorkflowID:  gr.workflow.ID,
		NodeID:      node.ID,
		Data: NodeCompleteData{
			Duration: time.Since(nodeStartTime),
			Result:   result.Data,
			Port:     result.Port,
		},
	})

	return result, nil
}

// parseBlockResult converts raw Bun output to BlockResult with port routing.
func (gr *GraphRunner) parseBlockResult(raw interface{}) (*BlockResult, error) {
	result := &BlockResult{
		Data: raw,
		Port: "default",
	}

	if resMap, ok := raw.(map[string]interface{}); ok {
		if port, hasPort := resMap["port"]; hasPort {
			if portStr, ok := port.(string); ok {
				result.Port = portStr
			}
		}

		if variables, hasVars := resMap["variables"]; hasVars {
			if err := gr.processVariableCommands(variables); err != nil {
				return nil, fmt.Errorf("failed to process variable commands: %w", err)
			}
		}

		if data, hasData := resMap["data"]; hasData {
			result.Data = data

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
	}

	return result, nil
}

func (gr *GraphRunner) processVariableCommands(variables interface{}) error {
	varList, ok := variables.([]interface{})
	if !ok {
		return nil
	}

	for _, v := range varList {
		cmd, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		action, _ := cmd["action"].(string)
		name, _ := cmd["name"].(string)

		if name == "" {
			continue
		}

		switch action {
		case "set":
			value := cmd["value"]
			options, _ := cmd["options"].(map[string]interface{})

			scope := ScopeExecution
			var ttl *time.Duration

			if options != nil {
				if scopeStr, ok := options["scope"].(string); ok {
					scope = VariableScope(scopeStr)
				}
				if ttlSec, ok := options["ttlSeconds"].(float64); ok && ttlSec > 0 {
					duration := time.Duration(ttlSec) * time.Second
					ttl = &duration
				}
			}

			gr.ctx.VariableStore.Set(gr.ctx.WorkflowID, gr.ctx.ExecutionID, name, value, scope, ttl)

		case "delete":
			gr.ctx.VariableStore.Delete(gr.ctx.WorkflowID, gr.ctx.ExecutionID, name)
		}
	}

	return nil
}

func (gr *GraphRunner) GetResults() map[string]interface{} {
	return gr.ctx.Results
}

// GetVariables returns all user-defined variables.
func (gr *GraphRunner) GetVariables() map[string]interface{} {
	return gr.ctx.Variables
}

func ResumeGraphExecution(ctx context.Context, store storage.Storage, executionID string, workflow *Workflow, blocksDir string) error {
	exec, err := store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution %s: %w", executionID, err)
	}

	var state resumeState
	if len(exec.State) == 0 {
		return fmt.Errorf("execution %s has no saved state", executionID)
	}
	if err := json.Unmarshal(exec.State, &state); err != nil {
		return fmt.Errorf("failed to parse execution state for %s: %w", executionID, err)
	}

	if state.CurrentNodeID == "" {
		return fmt.Errorf("execution %s has empty current_node_id", executionID)
	}

	if workflow.GetNode(state.CurrentNodeID) == nil {
		return fmt.Errorf("node %s not found in workflow %s", state.CurrentNodeID, workflow.ID)
	}

	runner := &GraphRunner{
		workflow:    workflow,
		bunRunner:   NewBunRunner(blocksDir),
		ctx:         NewExecutionContext(workflow.ID),
		storage:     store,
		executionID: executionID,
		lastNodeID:  state.CurrentNodeID,
	}

	runner.ctx.ExecutionID = executionID
	if state.Results != nil {
		runner.ctx.Results = state.Results
	}
	if state.Variables != nil {
		runner.ctx.Variables = state.Variables
	}

	var finalStatus = storage.ExecutionStatusCompleted
	var finalError *string

	defer func() {
		resume := resumeState{
			Results:       runner.ctx.Results,
			Variables:     runner.ctx.Variables,
			CurrentNodeID: runner.lastNodeID,
		}
		stateBytes, _ := json.Marshal(resume)
		if err := store.UpdateExecutionStatus(ctx, executionID, finalStatus, stateBytes, finalError); err != nil {
			log.Printf("Failed to update execution status during resume: %v", err)
		}
	}()

	if err := runner.executeFromNode(ctx, state.CurrentNodeID); err != nil {
		finalStatus = storage.ExecutionStatusFailed
		msg := err.Error()
		finalError = &msg
		return err
	}

	return nil
}
