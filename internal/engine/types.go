package engine

import (
	"sync"
	"time"
)

// NodeType defines the type of the node (e.g., "std/http_request", "trigger/http").
type NodeType string

const (
	NodeTypeHTTPRequest NodeType = "std/http_request"
	NodeTypeCustomCode  NodeType = "custom/code"
	NodeTypeCondition   NodeType = "std/condition"
	NodeTypeLoop        NodeType = "std/loop"
	NodeTypeTransform   NodeType = "std/transform"
	NodeTypeDelay       NodeType = "std/delay"
	NodeTypeFile        NodeType = "std/file"
	NodeTypeDatabase    NodeType = "std/database"
	NodeTypeWebhook     NodeType = "std/webhook"
	NodeTypeSetVar      NodeType = "std/set_var"
	NodeTypeGetVar      NodeType = "std/get_var"

	// Trigger nodes (long-running, emit events)
	NodeTypeTriggerHTTP      NodeType = "trigger/http"
	NodeTypeTriggerCron      NodeType = "trigger/cron"
	NodeTypeTriggerTelegram  NodeType = "trigger/telegram"
	NodeTypeTriggerWebSocket NodeType = "trigger/websocket"
)

// IsTrigger returns true if the node type is a long-running trigger.
func (nt NodeType) IsTrigger() bool {
	switch nt {
	case NodeTypeTriggerHTTP, NodeTypeTriggerCron, NodeTypeTriggerTelegram, NodeTypeTriggerWebSocket:
		return true
	default:
		return false
	}
}

// Position represents the visual position of a node in the editor.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Node represents a single block in the workflow graph.
type Node struct {
	ID       string         `json:"id"`
	Type     NodeType       `json:"type"`
	Position Position       `json:"position"`
	Config   map[string]any `json:"config,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// Edge represents a connection between two nodes.

type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`                 // Source node ID
	Target       string `json:"target"`                 // Target node ID
	SourceHandle string `json:"sourceHandle,omitempty"` // Output port (e.g., "true", "false", "default")
	TargetHandle string `json:"targetHandle,omitempty"` // Input port (e.g., "main", "data")
}

// Workflow represents the entire workflow as a graph of nodes and edges.

type Workflow struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Nodes  map[string]Node `json:"nodes"`
	Edges  []Edge          `json:"edges"`
	Config *WorkflowConfig `json:"config,omitempty"`
}

type WorkflowConfig struct {
	MaxConcurrentNodes int           `json:"max_concurrent_nodes,omitempty"`
	Timeout            time.Duration `json:"timeout,omitempty"`
	WorkerTimeout      time.Duration `json:"worker_timeout,omitempty"`
}

// GetNode returns a node by ID, or nil if not found.
func (w *Workflow) GetNode(id string) *Node {
	if node, ok := w.Nodes[id]; ok {
		return &node
	}
	return nil
}

// FindStartNodes returns all nodes that have no incoming edges (entry points).
func (w *Workflow) FindStartNodes() []string {
	hasIncoming := make(map[string]bool)
	for _, edge := range w.Edges {
		hasIncoming[edge.Target] = true
	}

	var startNodes []string
	for id := range w.Nodes {
		if !hasIncoming[id] {
			startNodes = append(startNodes, id)
		}
	}
	return startNodes
}

// FindNextNode finds the next node ID by following an edge from the given node and port.

func (w *Workflow) FindNextNode(nodeID, outputPort string) string {
	for _, edge := range w.Edges {
		if edge.Source == nodeID {

			if outputPort == "" || edge.SourceHandle == "" || edge.SourceHandle == outputPort {
				return edge.Target
			}
		}
	}
	return ""
}

// FindOutgoingEdges returns all edges originating from the given node.
func (w *Workflow) FindOutgoingEdges(nodeID string) []Edge {
	var edges []Edge
	for _, edge := range w.Edges {
		if edge.Source == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

type ErrorContext struct {
	Message   string    `json:"message"`
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
}

type ExecutionContext struct {
	WorkflowID    string
	ExecutionID   string
	Results       map[string]any
	resultsMu     sync.RWMutex
	Variables     map[string]any
	variablesMu   sync.RWMutex
	TriggerData   map[string]any
	VariableStore *VariableStore
	LastError     *ErrorContext
}

// NewExecutionContext creates a new context for a workflow execution.
func NewExecutionContext(workflowID string) *ExecutionContext {
	return &ExecutionContext{
		WorkflowID:    workflowID,
		Results:       make(map[string]any),
		Variables:     make(map[string]any),
		TriggerData:   make(map[string]any),
		VariableStore: NewVariableStore(),
	}
}

// SetResult saves the output of a node.
func (ctx *ExecutionContext) SetResult(nodeID string, result interface{}) {
	ctx.resultsMu.Lock()
	defer ctx.resultsMu.Unlock()
	ctx.Results[nodeID] = result
}

// GetResult retrieves the output of a node.
func (ctx *ExecutionContext) GetResult(nodeID string) interface{} {
	ctx.resultsMu.RLock()
	defer ctx.resultsMu.RUnlock()
	return ctx.Results[nodeID]
}

// SetVar sets a user-defined variable.
func (ctx *ExecutionContext) SetVar(name string, value interface{}) error {
	ctx.variablesMu.Lock()
	ctx.Variables[name] = value
	ctx.variablesMu.Unlock()
	return ctx.VariableStore.Set(ctx.WorkflowID, ctx.ExecutionID, name, value, ScopeExecution, nil)
}

func (ctx *ExecutionContext) GetVar(name string) interface{} {
	if value, ok := ctx.VariableStore.Get(ctx.WorkflowID, ctx.ExecutionID, name); ok {
		return value
	}
	return ctx.Variables[name]
}

func (ctx *ExecutionContext) SetError(nodeID, message, errorType string) {
	ctx.LastError = &ErrorContext{
		Message:   message,
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Type:      errorType,
	}
}

func (ctx *ExecutionContext) ClearError() {
	ctx.LastError = nil
}

// BlockResult represents the output from a Bun worker execution.

type NodeResult struct {
	Data any    `json:"data"`
	Port string `json:"port"`
}

func parseNodeResult(raw any) *NodeResult {
	result := &NodeResult{
		Data: raw,
		Port: "default",
	}

	resMap, ok := raw.(map[string]any)
	if !ok {
		return result
	}

	if port, hasPort := resMap["port"]; hasPort {
		if portStr, ok := port.(string); ok {
			result.Port = portStr
		}
	}

	if data, hasData := resMap["data"]; hasData {
		result.Data = data

		if dataMap, ok := data.(map[string]any); ok {
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
