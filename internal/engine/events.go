package engine

import "time"

type EventType string

const (
	EventTypeExecutionStart    EventType = "execution.start"
	EventTypeExecutionComplete EventType = "execution.complete"
	EventTypeExecutionError    EventType = "execution.error"
	EventTypeNodeStart         EventType = "node.start"
	EventTypeNodeComplete      EventType = "node.complete"
	EventTypeNodeError         EventType = "node.error"
)

type Event struct {
	Type        EventType
	Timestamp   time.Time
	ExecutionID string
	WorkflowID  string
	NodeID      string
	Data        interface{}
	Error       error
}

type ExecutionStartData struct {
	StartNodes []string
}

type ExecutionCompleteData struct {
	Duration time.Duration
	Results  map[string]interface{}
}

type NodeStartData struct {
	NodeType NodeType
	Config   map[string]interface{}
}

type NodeCompleteData struct {
	Duration time.Duration
	Result   interface{}
	Port     string
}

type EventListener interface {
	OnEvent(event Event)
}

type EventListenerFunc func(event Event)

func (f EventListenerFunc) OnEvent(event Event) {
	f(event)
}
