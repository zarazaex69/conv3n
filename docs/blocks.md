<div align="center">
  <img src="../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Standard Blocks

Conv3n includes built-in blocks for common operations.

### HTTP Request

Execute HTTP requests with automatic port routing based on status code.

```go
wf.AddNode(&conv3n.Node{
    ID:   "api_call",
    Type: "std/http_request",
    Config: map[string]interface{}{
        "url":    "https://api.example.com/data",
        "method": "POST",
        "headers": map[string]string{
            "Authorization": "Bearer token",
        },
        "body": map[string]interface{}{
            "key": "value",
        },
    },
})
```

**Output Ports:**
- `success` - 2xx status codes
- `client_error` - 4xx status codes
- `server_error` - 5xx status codes
- `error` - Network errors

### Transform

Transform data using JSONPath, field mapping, and expressions.

```go
wf.AddNode(&conv3n.Node{
    ID:   "transform",
    Type: "std/transform",
    Config: map[string]interface{}{
        "operations": []map[string]interface{}{
            {
                "type":   "pick",
                "fields": []string{"id", "name", "email"},
            },
            {
                "type":    "rename",
                "mapping": map[string]string{"email": "emailAddress"},
            },
            {
                "type":  "jsonpath",
                "query": "$.address.city",
            },
        },
    },
})
```

**Operations:**
- `pick` - Select specific fields
- `rename` - Rename object keys
- `map` - Transform with JavaScript expression
- `jsonpath` - Query with JSONPath (RFC 9535)

### Condition

Conditional branching with JavaScript expressions.

```go
wf.AddNode(&conv3n.Node{
    ID:   "check",
    Type: "std/condition",
    Config: map[string]interface{}{
        "expression": "input.status === 'active' && input.count > 10",
    },
})
```

**Output Ports:**
- `true` - Expression evaluates to true
- `false` - Expression evaluates to false

### Delay

Introduce time delays in workflow execution.

```go
wf.AddNode(&conv3n.Node{
    ID:   "wait",
    Type: "std/delay",
    Config: map[string]interface{}{
        "duration": 5,
        "unit":     "s",
    },
})
```

**Units:**
- `ms` - Milliseconds (default)
- `s` - Seconds

### Loop

Iterate over arrays with map and filter operations.

```go
wf.AddNode(&conv3n.Node{
    ID:   "process",
    Type: "std/loop",
    Config: map[string]interface{}{
        "items":            []interface{}{1, 2, 3, 4, 5},
        "filterExpression": "item > 2",
        "mapExpression":    "item * 2",
    },
})
```

**Output Ports:**
- `default` - Items processed successfully
- `empty` - No items after filtering

### Counter

Increment counters with variable scoping.

```go
wf.AddNode(&conv3n.Node{
    ID:   "count",
    Type: "std/counter",
    Config: map[string]interface{}{
        "counterName": "api_calls",
        "increment":   1,
        "scope":       "global",
        "ttlSeconds":  3600,
    },
})
```

**Scopes:**
- `global` - Shared across all workflows
- `workflow` - Shared across executions
- `execution` - Isolated to single execution

### Database

Execute SQLite operations.

```go
wf.AddNode(&conv3n.Node{
    ID:   "query",
    Type: "std/database",
    Config: map[string]interface{}{
        "database": "./data.db",
        "operation": map[string]interface{}{
            "type": "query",
            "sql":  "SELECT * FROM users WHERE active = ?",
            "params": []interface{}{true},
        },
    },
})
```

**Operations:**
- `query` - SELECT queries (returns rows)
- `execute` - INSERT/UPDATE/DELETE (returns changes)
- `transaction` - Multiple statements atomically

### File

File system operations.

```go
wf.AddNode(&conv3n.Node{
    ID:   "read",
    Type: "std/file",
    Config: map[string]interface{}{
        "path": "./data.json",
        "operation": map[string]interface{}{
            "type":   "read",
            "format": "json",
        },
    },
})
```

**Operations:**
- `read` - Read file (text/json/bytes)
- `write` - Write file
- `delete` - Delete file
- `exists` - Check file existence

### Webhook

Send outgoing webhooks.

```go
wf.AddNode(&conv3n.Node{
    ID:   "notify",
    Type: "std/webhook",
    Config: map[string]interface{}{
        "url":    "https://hooks.example.com/notify",
        "method": "POST",
        "body": map[string]interface{}{
            "event": "workflow_complete",
        },
        "timeout": 5000,
    },
})
```

**Methods:**
- `POST`
- `PUT`
- `PATCH`

### Variable Operations

Get and set variables in execution context.

```go
wf.AddNode(&conv3n.Node{
    ID:   "set",
    Type: "std/set_var",
    Config: map[string]interface{}{
        "name":  "result",
        "value": "success",
    },
})

wf.AddNode(&conv3n.Node{
    ID:   "get",
    Type: "std/get_var",
    Config: map[string]interface{}{
        "name": "result",
    },
})
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
