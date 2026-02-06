<div align="center">
  <img src="../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Core Concepts

### Architecture

Conv3n uses a hybrid Go + TypeScript architecture:

- **Go Engine**: Orchestrates workflow execution, manages state, handles concurrency
- **Bun Runtime**: Executes TypeScript blocks via worker pool
- **SQLite Storage**: Persists execution history and workflow definitions

### Workflow

A workflow is a directed acyclic graph (DAG) of nodes connected by edges.

```go
type Workflow struct {
    ID    string
    Name  string
    Nodes map[string]*Node
    Edges []*Edge
}
```

### Node

A node represents a single operation in the workflow.

```go
type Node struct {
    ID       string
    Type     string
    Position Position
    Config   map[string]interface{}
    Data     map[string]interface{}
}
```

**Node Types:**
- `std/http_request` - HTTP requests
- `std/transform` - Data transformation
- `std/condition` - Conditional branching
- `std/delay` - Time delays
- `std/loop` - Array iteration
- `std/counter` - Counter operations
- `std/database` - SQLite operations
- `std/file` - File system operations
- `std/webhook` - Outgoing webhooks
- `custom/*` - Your custom blocks

### Edge

An edge connects two nodes, defining execution flow.

```go
type Edge struct {
    ID           string
    Source       string
    Target       string
    SourceHandle string
    TargetHandle string
}
```

**Port Routing:**

Nodes can have multiple output ports for conditional routing:

```go
wf.AddEdge(&conv3n.Edge{
    Source:       "condition",
    Target:       "success_handler",
    SourceHandle: "true",
})

wf.AddEdge(&conv3n.Edge{
    Source:       "condition",
    Target:       "failure_handler",
    SourceHandle: "false",
})
```

### Execution Context

Each workflow execution has an isolated context:

```go
type ExecutionContext struct {
    ExecutionID string
    WorkflowID  string
    TriggerData map[string]interface{}
    Results     map[string]interface{}
    Variables   *VariableStore
}
```

### Variable Scopes

Variables can have different scopes:

- **Global**: Shared across all workflows and executions
- **Workflow**: Shared across executions of the same workflow
- **Execution**: Isolated to a single execution (default)

```typescript
variables.setGlobal("api_key", "secret123")
variables.setWorkflow("counter", 0)
variables.setExecution("temp", data)
```

### Runtime Versions

**Runtime (Production):**
- Advanced worker pool with health checks
- Circuit breaker + retry logic
- Graceful shutdown
- Observability (metrics, tracing, logging)
- Rate limiting

```go
runtime, err := conv3n.New(cfg)
```

### Execution Lifecycle

1. **Start**: Runtime initializes worker pool and storage
2. **Execute**: Workflow submitted for execution
3. **Run**: Nodes execute in topological order
4. **Complete**: Results persisted to storage
5. **Stop**: Graceful shutdown with cleanup

### Error Handling

Errors are propagated through the execution chain:

```go
handle, err := runtime.Execute(ctx, wf, nil)
if err != nil {
    return err
}

if err := handle.Wait(ctx); err != nil {
    log.Printf("Execution failed: %v", err)
}
```

### Concurrency

Conv3n executes independent nodes in parallel:

```
     ┌─────┐
     │Start│
     └──┬──┘
   ┌────┴────┐
   │         │
┌──▼──┐   ┌─▼───┐
│Node1│   │Node2│  (parallel)
└──┬──┘   └─┬───┘
   └────┬────┘
     ┌──▼──┐
     │ End │
     └─────┘
```

Worker pool size controls max parallelism:

```go
cfg.WorkerPoolSize = 8
```

### Observability

Runtime provides built-in observability:

```go
health := runtime.Health(ctx)
metrics := runtime.Metrics()
traces := runtime.Traces()
```

### Execution Management

Query and manage executions:

```go
status, err := runtime.GetExecution(ctx, executionID)

executions, err := runtime.ListExecutions(ctx, workflowID, 10)

handle, err := runtime.Execute(ctx, wf, nil)
if err := handle.Wait(ctx); err != nil {
    log.Printf("Execution failed: %v", err)
}
```

### Graceful Shutdown

Wait for shutdown signals:

```go
runtime.WaitForShutdown(ctx)
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
