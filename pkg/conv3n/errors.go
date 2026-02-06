package conv3n

import "errors"

var (
	ErrAlreadyRunning     = errors.New("runtime is already running")
	ErrNotRunning         = errors.New("runtime is not running")
	ErrExecutionCancelled = errors.New("execution was cancelled")
	ErrWorkflowNotFound   = errors.New("workflow not found")
	ErrExecutionNotFound  = errors.New("execution not found")
	ErrInvalidWorkflow    = errors.New("invalid workflow definition")
	ErrNodeNotFound       = errors.New("node not found")
)
