package conv3n

type EventHandler interface {
	OnExecutionStart(execID, workflowID string)
	OnExecutionComplete(execID string, err error)
	OnExecutionStop(execID string)
	OnNodeExecute(execID, nodeID string, result map[string]interface{})
}

type NoOpEventHandler struct{}

func (h *NoOpEventHandler) OnExecutionStart(execID, workflowID string)                         {}
func (h *NoOpEventHandler) OnExecutionComplete(execID string, err error)                       {}
func (h *NoOpEventHandler) OnExecutionStop(execID string)                                      {}
func (h *NoOpEventHandler) OnNodeExecute(execID, nodeID string, result map[string]interface{}) {}
