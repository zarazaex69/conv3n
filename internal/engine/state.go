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

// GetResult retrieves the output of a block.
func (sm *StateManager) GetResult(blockID string) interface{} {
	return sm.ctx.Results[blockID]
}

// PrepareInput creates the input payload for a block, resolving any variables in the config.
func (sm *StateManager) PrepareInput(block Block) (map[string]interface{}, error) {

	// We pass the entire ExecutionContext for access to both Results and Variables
	resolvedConfig, err := ResolveVariables(block.Config, sm.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve variables in config: %w", err)
	}

	return map[string]interface{}{
		"config": resolvedConfig,

		// "context": sm.ctx.Results,
	}, nil
}
