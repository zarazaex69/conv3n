# Conv3n Go SDK

Embeddable workflow execution runtime for Go applications.

## Installation

```bash
go get github.com/zarazaex69/conv3n/pkg/conv3n
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/zarazaex69/conv3n/pkg/conv3n"
)

func main() {
    cfg := conv3n.DefaultConfig()
    cfg.BlocksDir = "pkg/blocks"
    cfg.StoragePath = "workflows.db"
    cfg.MaxWorkers = 5
    
    runtime, err := conv3n.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer runtime.Close()
    
    ctx := context.Background()
    if err := runtime.Start(ctx); err != nil {
        log.Fatal(err)
    }
    
    wf := conv3n.NewWorkflow("wf_1", "My Workflow")
    wf.AddNode(&conv3n.Node{
        ID:   "node_1",
        Type: "std/http_request",
        Config: map[string]interface{}{
            "url":    "https://api.example.com/data",
            "method": "GET",
        },
    })
    
    handle, err := runtime.Execute(ctx, wf, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := handle.Wait(ctx); err != nil {
        log.Fatal(err)
    }
    
    status, _ := handle.Status(ctx)
    log.Printf("Execution completed: %s", status.Status)
}
```

## Core Concepts

### Runtime

The `Runtime` is the main entry point for embedding Conv3n. It manages:
- Worker pool for concurrent execution
- Storage backend (SQLite by default)
- Execution lifecycle
- Event callbacks

```go
runtime, err := conv3n.New(&conv3n.Config{
    BlocksDir:    "pkg/blocks",
    StoragePath:  "conv3n.db",
    MaxWorkers:   10,
    EventHandler: &MyEventHandler{},
})
```

### Workflow

A `Workflow` is a directed graph of nodes (blocks) connected by edges:

```go
wf := conv3n.NewWorkflow("wf_1", "Data Pipeline")

wf.AddNode(&conv3n.Node{
    ID:   "fetch",
    Type: "std/http_request",
    Config: map[string]interface{}{
        "url": "https://api.example.com/users",
    },
})

wf.AddNode(&conv3n.Node{
    ID:   "transform",
    Type: "std/transform",
    Config: map[string]interface{}{
        "code": "return input.data.map(u => u.name)",
    },
})

wf.AddEdge(&conv3n.Edge{
    ID:     "e1",
    Source: "fetch",
    Target: "transform",
})
```

### Execution

Execute workflows and track their progress:

```go
handle, err := runtime.Execute(ctx, wf, map[string]interface{}{
    "user_id": 123,
})

go func() {
    if err := handle.Wait(ctx); err != nil {
        log.Printf("Execution failed: %v", err)
    }
}()

status, _ := handle.Status(ctx)
result, _ := handle.GetNodeResult(ctx, "transform")
```

### Events

Implement `EventHandler` to monitor execution lifecycle:

```go
type MyHandler struct{}

func (h *MyHandler) OnExecutionStart(execID, workflowID string) {
    log.Printf("Started: %s", execID)
}

func (h *MyHandler) OnExecutionComplete(execID string, err error) {
    if err != nil {
        log.Printf("Failed: %s - %v", execID, err)
    } else {
        log.Printf("Completed: %s", execID)
    }
}

func (h *MyHandler) OnExecutionStop(execID string) {
    log.Printf("Stopped: %s", execID)
}

func (h *MyHandler) OnNodeExecute(execID, nodeID string, result map[string]interface{}) {
    log.Printf("Node %s executed: %v", nodeID, result)
}
```

## Loading Workflows from JSON

```go
data, _ := os.ReadFile("workflow.json")
wf, err := conv3n.LoadWorkflowFromJSON(data)
if err != nil {
    log.Fatal(err)
}

handle, _ := runtime.Execute(ctx, wf, nil)
```

## Execution Control

```go
handle, _ := runtime.Execute(ctx, wf, nil)

if err := handle.Stop(); err != nil {
    log.Printf("Failed to stop: %v", err)
}

status, _ := handle.Status(ctx)
state, _ := handle.GetState(ctx)
nodeResult, _ := handle.GetNodeResult(ctx, "node_1")
```

## Thread Safety

All `Runtime` methods are thread-safe and can be called concurrently from multiple goroutines.

## Error Handling

```go
var (
    ErrAlreadyRunning     = errors.New("runtime is already running")
    ErrNotRunning         = errors.New("runtime is not running")
    ErrExecutionCancelled = errors.New("execution was cancelled")
    ErrWorkflowNotFound   = errors.New("workflow not found")
    ErrExecutionNotFound  = errors.New("execution not found")
    ErrInvalidWorkflow    = errors.New("invalid workflow definition")
    ErrNodeNotFound       = errors.New("node not found")
)
```

## Architecture

```
┌─────────────────────────────────────────┐
│           Your Application              │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│         Conv3n Runtime (Go)             │
│  ┌─────────────────────────────────┐   │
│  │      Execution Registry         │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │        Worker Pool              │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │      Storage (SQLite)           │   │
│  └─────────────────────────────────┘   │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│      Bun Runtime (TypeScript)           │
│  ┌─────────────────────────────────┐   │
│  │    Block Execution (stdin/out)  │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

## License

See LICENSE file.
