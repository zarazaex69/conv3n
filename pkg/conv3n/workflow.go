package conv3n

import (
	"encoding/json"
	"fmt"

	"github.com/zarazaex69/conv3n/internal/engine"
)

type Workflow struct {
	ID     string
	Name   string
	Nodes  map[string]*Node
	Edges  []*Edge
	Config *WorkflowConfig
}

type WorkflowConfig struct {
	MaxConcurrentNodes int
}

type Node struct {
	ID       string
	Type     string
	Position Position
	Config   map[string]any
	Data     map[string]any
}

type Edge struct {
	ID           string
	Source       string
	Target       string
	SourceHandle string
	TargetHandle string
}

type Position struct {
	X float64
	Y float64
}

func NewWorkflow(id, name string) *Workflow {
	return &Workflow{
		ID:     id,
		Name:   name,
		Nodes:  make(map[string]*Node),
		Edges:  make([]*Edge, 0),
		Config: &WorkflowConfig{},
	}
}

func (w *Workflow) AddNode(node *Node) {
	w.Nodes[node.ID] = node
}

func (w *Workflow) AddEdge(edge *Edge) {
	w.Edges = append(w.Edges, edge)
}

func (w *Workflow) toEngine() *engine.Workflow {
	nodes := make(map[string]engine.Node)
	for id, n := range w.Nodes {
		nodes[id] = engine.Node{
			ID:       n.ID,
			Type:     engine.NodeType(n.Type),
			Position: engine.Position{X: n.Position.X, Y: n.Position.Y},
			Config:   n.Config,
			Data:     n.Data,
		}
	}

	edges := make([]engine.Edge, len(w.Edges))
	for i, e := range w.Edges {
		edges[i] = engine.Edge{
			ID:           e.ID,
			Source:       e.Source,
			Target:       e.Target,
			SourceHandle: e.SourceHandle,
			TargetHandle: e.TargetHandle,
		}
	}

	var config *engine.WorkflowConfig
	if w.Config != nil {
		config = &engine.WorkflowConfig{
			MaxConcurrentNodes: w.Config.MaxConcurrentNodes,
		}
	}

	return &engine.Workflow{
		ID:     w.ID,
		Name:   w.Name,
		Nodes:  nodes,
		Edges:  edges,
		Config: config,
	}
}

func LoadWorkflowFromJSON(data []byte) (*Workflow, error) {
	var engineWf engine.Workflow
	if err := json.Unmarshal(data, &engineWf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	nodes := make(map[string]*Node)
	for id, n := range engineWf.Nodes {
		nodes[id] = &Node{
			ID:       n.ID,
			Type:     string(n.Type),
			Position: Position{X: n.Position.X, Y: n.Position.Y},
			Config:   n.Config,
			Data:     n.Data,
		}
	}

	edges := make([]*Edge, len(engineWf.Edges))
	for i, e := range engineWf.Edges {
		edges[i] = &Edge{
			ID:           e.ID,
			Source:       e.Source,
			Target:       e.Target,
			SourceHandle: e.SourceHandle,
			TargetHandle: e.TargetHandle,
		}
	}

	var config *WorkflowConfig
	if engineWf.Config != nil {
		config = &WorkflowConfig{
			MaxConcurrentNodes: engineWf.Config.MaxConcurrentNodes,
		}
	}

	return &Workflow{
		ID:     engineWf.ID,
		Name:   engineWf.Name,
		Nodes:  nodes,
		Edges:  edges,
		Config: config,
	}, nil
}

func (w *Workflow) ToJSON() ([]byte, error) {
	engineWf := w.toEngine()
	return json.Marshal(engineWf)
}
