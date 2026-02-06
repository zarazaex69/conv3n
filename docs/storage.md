<div align="center">
  <img src="../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Storage

Conv3n uses SQLite for execution history and workflow persistence.

### Execution History

Query executions via Runtime API:

```go
runtime, err := conv3n.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer runtime.Close()

executions, err := runtime.ListExecutions(ctx, workflowID, 10)
for _, exec := range executions {
    log.Printf("Execution: %s, Status: %s, Started: %s",
        exec.ID, exec.Status, exec.StartedAt)
}

status, err := runtime.GetExecution(ctx, executionID)
if err != nil {
    log.Fatal(err)
}
log.Printf("Status: %s", status.Status)
```

### Execution Status

```go
type ExecutionState string

const (
    ExecutionStatePending   ExecutionState = "pending"
    ExecutionStateRunning   ExecutionState = "running"
    ExecutionStateCompleted ExecutionState = "completed"
    ExecutionStateFailed    ExecutionState = "failed"
    ExecutionStateCancelled ExecutionState = "cancelled"
)
```

### Node Results

Retrieve results from specific nodes via ExecutionHandle:

```go
handle, err := runtime.Execute(ctx, wf, nil)
if err != nil {
    log.Fatal(err)
}

if err := handle.Wait(ctx); err != nil {
    log.Fatal(err)
}

result, err := handle.GetNodeResult(ctx, "transform")
if err != nil {
    log.Fatal(err)
}

log.Printf("Transform output: %+v", result)
```

### Execution State

Get full execution state:

```go
state, err := handle.GetState(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Execution state: %+v", state)
```

### Workflow Persistence

Store workflow definitions:

```go
import "github.com/zarazaex69/conv3n/internal/storage"

store, _ := storage.NewSQLite("conv3n.db")

workflowJSON, _ := wf.ToJSON()

err := store.CreateWorkflow(ctx, &storage.Workflow{
    ID:         wf.ID,
    Name:       wf.Name,
    Definition: workflowJSON,
})

savedWf, err := store.GetWorkflow(ctx, wf.ID)
```

### Database Schema

**workflow_executions:**
```sql
CREATE TABLE workflow_executions (
    execution_id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    status TEXT NOT NULL,
    state BLOB NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    error TEXT
);
```

**node_results:**
```sql
CREATE TABLE node_results (
    execution_id TEXT NOT NULL,
    node_id TEXT NOT NULL,
    result BLOB NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (execution_id, node_id)
);
```

**workflows:**
```sql
CREATE TABLE workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    definition BLOB NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Query Execution History

```go
executions, err := runtime.ListExecutions(ctx, workflowID, 100)

for _, exec := range executions {
    if exec.Status == conv3n.ExecutionStateFailed {
        log.Printf("Failed execution: %s, Error: %s",
            exec.ID, *exec.Error)
    }
}
```

### Storage Interface

Implement custom storage backends:

```go
type Storage interface {
    CreateWorkflow(ctx context.Context, workflow *Workflow) error
    GetWorkflow(ctx context.Context, id string) (*Workflow, error)
    UpdateWorkflow(ctx context.Context, workflow *Workflow) error
    DeleteWorkflow(ctx context.Context, id string) error
    ListWorkflows(ctx context.Context) ([]*Workflow, error)

    CreateExecution(ctx context.Context, workflowID string) (executionID string, err error)
    UpdateExecutionStatus(ctx context.Context, executionID string, status ExecutionStatus, state []byte, errorMsg *string) error
    GetExecution(ctx context.Context, executionID string) (*Execution, error)
    ListExecutions(ctx context.Context, workflowID string, limit int) ([]*Execution, error)

    SaveNodeResult(ctx context.Context, executionID, nodeID string, result []byte) error
    GetNodeResult(ctx context.Context, executionID, nodeID string) ([]byte, error)

    Close() error
}
```

### In-Memory Storage

For testing, use in-memory database:

```go
store, err := storage.NewSQLite(":memory:")
```

### Cleanup Old Executions

```sql
DELETE FROM workflow_executions 
WHERE started_at < datetime('now', '-30 days');
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
