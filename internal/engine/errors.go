package engine

import (
	"errors"
	"fmt"
)

var (
	ErrNodeNotFound      = errors.New("node not found in workflow")
	ErrInvalidNodeType   = errors.New("invalid or unknown node type")
	ErrScriptNotFound    = errors.New("block script not found")
	ErrExecutionTimeout  = errors.New("execution timeout exceeded")
	ErrValidationFailed  = errors.New("workflow validation failed")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrWorkerUnavailable = errors.New("no healthy workers available")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error [%s]: %s", e.Field, e.Message)
}

type NodeExecutionError struct {
	NodeID   string
	NodeType NodeType
	Err      error
}

func (e *NodeExecutionError) Error() string {
	return fmt.Sprintf("node %s (%s) failed: %v", e.NodeID, e.NodeType, e.Err)
}

func (e *NodeExecutionError) Unwrap() error {
	return e.Err
}

func NewNodeExecutionError(nodeID string, nodeType NodeType, err error) error {
	return &NodeExecutionError{
		NodeID:   nodeID,
		NodeType: nodeType,
		Err:      err,
	}
}
