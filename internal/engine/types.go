package engine

import "fmt"

// =============================================================================
// NODE TYPES
// =============================================================================

// NodeType defines the type of the node (e.g., "std/http_request", "trigger/http").
type NodeType string

const (
	// Action nodes (execute and exit)
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

// =============================================================================
// GRAPH STRUCTURES
// =============================================================================

// Position represents the visual position of a node in the editor.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Node represents a single block in the workflow graph.
type Node struct {
	ID       string                 `json:"id"`
	Type     NodeType               `json:"type"`
	Position Position               `json:"position"`
	Config   map[string]interface{} `json:"config,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// Edge represents a connection between two nodes.
// Supports multiple output ports (e.g., "true"/"false" for conditions).
type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`                 // Source node ID
	Target       string `json:"target"`                 // Target node ID
	SourceHandle string `json:"sourceHandle,omitempty"` // Output port (e.g., "true", "false", "default")
	TargetHandle string `json:"targetHandle,omitempty"` // Input port (e.g., "main", "data")
}

// =============================================================================
// WORKFLOW (Graph-based)
// =============================================================================

// Workflow represents the entire workflow as a graph of nodes and edges.
// This is the new graph-based structure replacing the linear []Block array.
type Workflow struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Nodes map[string]Node `json:"nodes"` // Node ID -> Node
	Edges []Edge          `json:"edges"`
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
// Returns empty string if no matching edge is found (end of execution path).
func (w *Workflow) FindNextNode(nodeID, outputPort string) string {
	for _, edge := range w.Edges {
		if edge.Source == nodeID {
			// If outputPort is specified, match it; otherwise match any edge from this node
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

// =============================================================================
// EXECUTION CONTEXT
// =============================================================================

// ExecutionContext holds the state of a running workflow execution.
type ExecutionContext struct {
	WorkflowID  string
	ExecutionID string
	// Results stores the output of each node by Node ID
	Results map[string]interface{}
	// Variables stores user-defined variables (mutable state)
	Variables map[string]interface{}
	// TriggerData stores the payload from the trigger (e.g. webhook body)
	TriggerData map[string]interface{}
}

// NewExecutionContext creates a new context for a workflow execution.
func NewExecutionContext(workflowID string) *ExecutionContext {
	return &ExecutionContext{
		WorkflowID:  workflowID,
		Results:     make(map[string]interface{}),
		Variables:   make(map[string]interface{}),
		TriggerData: make(map[string]interface{}),
	}
}

// SetResult saves the output of a node.
func (ctx *ExecutionContext) SetResult(nodeID string, result interface{}) {
	ctx.Results[nodeID] = result
}

// GetResult retrieves the output of a node.
func (ctx *ExecutionContext) GetResult(nodeID string) interface{} {
	return ctx.Results[nodeID]
}

// SetVar sets a user-defined variable.
func (ctx *ExecutionContext) SetVar(name string, value interface{}) {
	ctx.Variables[name] = value
}

// GetVar retrieves a user-defined variable.
func (ctx *ExecutionContext) GetVar(name string) interface{} {
	return ctx.Variables[name]
}

// =============================================================================
// BLOCK RESULT (Output from Bun workers)
// =============================================================================

// BlockResult represents the output from a Bun worker execution.
// Includes both the data and the output port for routing.
type BlockResult struct {
	Data interface{} `json:"data"` // Output data
	Port string      `json:"port"` // Output port name (e.g., "default", "true", "false")
}

// =============================================================================
// LEGACY TYPES (for backward compatibility during migration)
// =============================================================================

// BlockType is an alias for NodeType (legacy compatibility).
type BlockType = NodeType

// Legacy constants for backward compatibility with existing tests.
const (
	BlockTypeHTTPRequest = NodeTypeHTTPRequest
	BlockTypeCustomCode  = NodeTypeCustomCode
	BlockTypeCondition   = NodeTypeCondition
	BlockTypeLoop        = NodeTypeLoop
	BlockTypeTransform   = NodeTypeTransform
)

// Block is the legacy structure for linear workflows.
// Deprecated: Use Node instead.
type Block struct {
	ID     string                 `json:"id"`
	Type   BlockType              `json:"type"`
	Config map[string]interface{} `json:"config"`
	Code   string                 `json:"code,omitempty"`
}

// Connection is the legacy structure for simple connections.
// Deprecated: Use Edge instead.
type Connection struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// LegacyWorkflow is the old linear workflow structure.
// Used for migration from old format to new graph format.
type LegacyWorkflow struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Blocks      []Block      `json:"blocks"`
	Connections []Connection `json:"connections"`
}

// ToGraphWorkflow converts a legacy linear workflow to the new graph format.
func (lw *LegacyWorkflow) ToGraphWorkflow() *Workflow {
	nodes := make(map[string]Node)
	var edges []Edge

	// Convert blocks to nodes with auto-positioned layout
	for i, block := range lw.Blocks {
		nodes[block.ID] = Node{
			ID:   block.ID,
			Type: NodeType(block.Type),
			Position: Position{
				X: float64(i * 250), // Horizontal layout
				Y: 100,
			},
			Config: block.Config,
			Data: map[string]interface{}{
				"label": block.ID,
			},
		}
	}

	// Convert connections to edges
	for i, conn := range lw.Connections {
		edges = append(edges, Edge{
			ID:           fmt.Sprintf("e%d", i),
			Source:       conn.From,
			Target:       conn.To,
			SourceHandle: "default",
			TargetHandle: "main",
		})
	}

	// If no explicit connections, create linear chain
	if len(edges) == 0 && len(lw.Blocks) > 1 {
		for i := 0; i < len(lw.Blocks)-1; i++ {
			edges = append(edges, Edge{
				ID:           fmt.Sprintf("e%d", i),
				Source:       lw.Blocks[i].ID,
				Target:       lw.Blocks[i+1].ID,
				SourceHandle: "default",
				TargetHandle: "main",
			})
		}
	}

	return &Workflow{
		ID:    lw.ID,
		Name:  lw.Name,
		Nodes: nodes,
		Edges: edges,
	}
}
