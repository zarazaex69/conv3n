package engine

import (
	"fmt"
)

type WorkflowValidator struct {
	registry *BlockRegistry
}

func NewWorkflowValidator(registry *BlockRegistry) *WorkflowValidator {
	return &WorkflowValidator{
		registry: registry,
	}
}

func (v *WorkflowValidator) Validate(wf *Workflow) error {
	if wf.ID == "" {
		return &ValidationError{Field: "id", Message: "workflow ID cannot be empty"}
	}

	if len(wf.Nodes) == 0 {
		return &ValidationError{Field: "nodes", Message: "workflow must have at least one node"}
	}

	if err := v.validateNodes(wf); err != nil {
		return err
	}

	if err := v.validateEdges(wf); err != nil {
		return err
	}

	if err := v.detectCycles(wf); err != nil {
		return err
	}

	startNodes := wf.FindStartNodes()
	if len(startNodes) == 0 {
		return &ValidationError{Field: "nodes", Message: "workflow must have at least one start node (node with no incoming edges)"}
	}

	return nil
}

func (v *WorkflowValidator) validateNodes(wf *Workflow) error {
	for id, node := range wf.Nodes {
		if node.ID == "" {
			return &ValidationError{Field: fmt.Sprintf("nodes[%s].id", id), Message: "node ID cannot be empty"}
		}

		if node.Type == "" {
			return &ValidationError{Field: fmt.Sprintf("nodes[%s].type", id), Message: "node type cannot be empty"}
		}

		if node.Type != NodeTypeSetVar && node.Type != NodeTypeGetVar {
			if _, ok := v.registry.Get(node.Type); !ok {
				return &ValidationError{
					Field:   fmt.Sprintf("nodes[%s].type", id),
					Message: fmt.Sprintf("unknown node type: %s", node.Type),
				}
			}
		}
	}

	return nil
}

func (v *WorkflowValidator) validateEdges(wf *Workflow) error {
	for i, edge := range wf.Edges {
		if edge.Source == "" {
			return &ValidationError{Field: fmt.Sprintf("edges[%d].source", i), Message: "edge source cannot be empty"}
		}

		if edge.Target == "" {
			return &ValidationError{Field: fmt.Sprintf("edges[%d].target", i), Message: "edge target cannot be empty"}
		}

		if _, ok := wf.Nodes[edge.Source]; !ok {
			return &ValidationError{
				Field:   fmt.Sprintf("edges[%d].source", i),
				Message: fmt.Sprintf("source node %s does not exist", edge.Source),
			}
		}

		if _, ok := wf.Nodes[edge.Target]; !ok {
			return &ValidationError{
				Field:   fmt.Sprintf("edges[%d].target", i),
				Message: fmt.Sprintf("target node %s does not exist", edge.Target),
			}
		}
	}

	return nil
}

func (v *WorkflowValidator) detectCycles(wf *Workflow) error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(nodeID string) bool
	hasCycle = func(nodeID string) bool {
		visited[nodeID] = true
		recStack[nodeID] = true

		for _, edge := range wf.Edges {
			if edge.Source == nodeID {
				if !visited[edge.Target] {
					if hasCycle(edge.Target) {
						return true
					}
				} else if recStack[edge.Target] {
					return true
				}
			}
		}

		recStack[nodeID] = false
		return false
	}

	for nodeID := range wf.Nodes {
		if !visited[nodeID] {
			if hasCycle(nodeID) {
				return &ValidationError{Field: "edges", Message: "workflow contains cycles"}
			}
		}
	}

	return nil
}
