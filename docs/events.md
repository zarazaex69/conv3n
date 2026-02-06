<div align="center">
  <img src="../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Event Handling

Monitor workflow execution with event handlers.

### EventHandler Interface

```go
type EventHandler interface {
    OnExecutionStart(execID, workflowID string)
    OnExecutionComplete(execID string, err error)
    OnExecutionStop(execID string)
    OnNodeExecute(execID, nodeID string, result map[string]interface{})
}
```

### Basic Example

```go
type LoggingHandler struct{}

func (h *LoggingHandler) OnExecutionStart(execID, workflowID string) {
    log.Printf("[START] Execution %s for workflow %s", execID, workflowID)
}

func (h *LoggingHandler) OnExecutionComplete(execID string, err error) {
    if err != nil {
        log.Printf("[ERROR] Execution %s failed: %v", execID, err)
    } else {
        log.Printf("[COMPLETE] Execution %s finished", execID)
    }
}

func (h *LoggingHandler) OnExecutionStop(execID string) {
    log.Printf("[STOP] Execution %s was stopped", execID)
}

func (h *LoggingHandler) OnNodeExecute(execID, nodeID string, result map[string]interface{}) {
    log.Printf("[NODE] %s executed in %s", nodeID, execID)
}

cfg := conv3n.DefaultConfigV2()
cfg.EventHandler = &LoggingHandler{}
```

### Advanced Event Listener

For more detailed events, use the internal engine event system:

```go
import "github.com/zarazaex69/conv3n/internal/engine"

type DetailedListener struct{}

func (l *DetailedListener) OnEvent(event engine.Event) {
    switch event.Type {
    case engine.EventTypeExecutionStart:
        data := event.Data.(engine.ExecutionStartData)
        log.Printf("Execution started with %d start nodes", len(data.StartNodes))

    case engine.EventTypeNodeStart:
        data := event.Data.(engine.NodeStartData)
        log.Printf("Node %s started (type: %s)", event.NodeID, data.NodeType)

    case engine.EventTypeNodeComplete:
        data := event.Data.(engine.NodeCompleteData)
        log.Printf("Node %s completed in %s (port: %s)", 
            event.NodeID, data.Duration, data.Port)

    case engine.EventTypeNodeError:
        log.Printf("Node %s failed: %v", event.NodeID, event.Error)

    case engine.EventTypeExecutionComplete:
        data := event.Data.(engine.ExecutionCompleteData)
        log.Printf("Execution completed in %s", data.Duration)

    case engine.EventTypeExecutionError:
        log.Printf("Execution failed: %v", event.Error)
    }
}

runner := engine.NewGraphRunner(workflow, "pkg/blocks", store)
runner.SetEventListener(&DetailedListener{})
```

### Event Types

**Execution Events:**
- `EventTypeExecutionStart` - Workflow execution started
- `EventTypeExecutionComplete` - Workflow execution completed
- `EventTypeExecutionError` - Workflow execution failed

**Node Events:**
- `EventTypeNodeStart` - Node execution started
- `EventTypeNodeComplete` - Node execution completed
- `EventTypeNodeError` - Node execution failed

### Event Data Structures

```go
type ExecutionStartData struct {
    StartNodes []string
}

type ExecutionCompleteData struct {
    Duration time.Duration
}

type NodeStartData struct {
    NodeType engine.NodeType
}

type NodeCompleteData struct {
    Duration time.Duration
    Port     string
}
```

### Metrics Collection

Collect metrics from events:

```go
type MetricsHandler struct {
    executions int64
    failures   int64
    mu         sync.Mutex
}

func (h *MetricsHandler) OnExecutionStart(execID, workflowID string) {
    h.mu.Lock()
    h.executions++
    h.mu.Unlock()
}

func (h *MetricsHandler) OnExecutionComplete(execID string, err error) {
    if err != nil {
        h.mu.Lock()
        h.failures++
        h.mu.Unlock()
    }
}

func (h *MetricsHandler) Stats() (executions, failures int64) {
    h.mu.Lock()
    defer h.mu.Unlock()
    return h.executions, h.failures
}
```

### Alerting

Send alerts on failures:

```go
type AlertHandler struct {
    webhookURL string
}

func (h *AlertHandler) OnExecutionComplete(execID string, err error) {
    if err != nil {
        payload := map[string]interface{}{
            "execution_id": execID,
            "error":        err.Error(),
            "timestamp":    time.Now(),
        }
        
        go h.sendAlert(payload)
    }
}

func (h *AlertHandler) sendAlert(payload map[string]interface{}) {
    body, _ := json.Marshal(payload)
    http.Post(h.webhookURL, "application/json", bytes.NewReader(body))
}
```

### NoOp Handler

Use NoOpEventHandler when you don't need events:

```go
cfg.EventHandler = &conv3n.NoOpEventHandler{}
```

<div align="center">

---

### Contact

Telegram: [zarazaex](https://t.me/zarazaexe)
<br>
Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io)
<br>
Site: [zarazaex.xyz](https://zarazaex.xyz)

</div>
