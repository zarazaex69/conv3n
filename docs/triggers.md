<div align="center">
  <img src="../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Triggers

Triggers automatically start workflow executions based on events.

### Standard Triggers

**Cron:**
```typescript
{
    "type": "cron",
    "config": {
        "schedule": "0 */5 * * * *"
    }
}
```

**Webhook:**
```typescript
{
    "type": "webhook",
    "config": {}
}
```

### Custom Triggers

**HTTP Server:**
```typescript
{
    "type": "custom/http_server",
    "config": {
        "port": 3000,
        "path": "/webhook"
    }
}
```

**Command Watch:**
```typescript
{
    "type": "custom/command_watch",
    "config": {
        "command": "git",
        "args": ["status", "--porcelain"],
        "interval": 5000,
        "matchPattern": "^M"
    }
}
```

### Trigger Management

```go
import "github.com/zarazaex69/conv3n/internal/storage"

store, _ := storage.NewSQLite("conv3n.db")

trigger := &storage.Trigger{
    ID:         "cron-daily",
    WorkflowID: "backup-workflow",
    Type:       "cron",
    Config:     []byte(`{"schedule":"0 0 * * *"}`),
    Enabled:    true,
}

err := store.CreateTrigger(ctx, trigger)
```

### Trigger Execution History

```go
executions, err := store.ListTriggerExecutions(ctx, triggerID, 100)

for _, exec := range executions {
    log.Printf("Fired at: %s, Status: %s", exec.FiredAt, exec.Status)
}
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
