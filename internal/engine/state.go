package engine

import (
	"fmt"
)

// StateManager handles the data flow between blocks.
type StateManager struct {
	ctx *ExecutionContext
}

func NewStateManager(ctx *ExecutionContext) *StateManager {
	return &StateManager{ctx: ctx}
}

// SetResult saves the output of a block.
func (sm *StateManager) SetResult(blockID string, result interface{}) {
	sm.ctx.Results[blockID] = result
}

func (sm *StateManager) GetResult(nodeID string) interface{} {
	return sm.ctx.Results[nodeID]
}

func (sm *StateManager) PrepareInput(node *Node) (map[string]interface{}, error) {
	resolvedConfig, err := ResolveVariables(node.Config, sm.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve variables in config: %w", err)
	}

	return map[string]interface{}{
		"config": resolvedConfig,

		// "context": sm.ctx.Results,
	}, nil
}
