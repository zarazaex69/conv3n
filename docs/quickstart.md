<div align="center">
  <img src="../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Quick Start

Conv3n is an embeddable workflow automation engine for Go applications. This guide will get you running in 5 minutes.

### Installation

```bash
go get github.com/zarazaex69/conv3n
```

### Basic Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/zarazaex69/conv3n/pkg/conv3n"
)

func main() {
    cfg := conv3n.DefaultConfigV2()
    cfg.BlocksDir = "pkg/blocks"
    cfg.StoragePath = "conv3n.db"
    cfg.WorkerPoolSize = 4

    runtime, err := conv3n.NewV2(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer runtime.Close()

    ctx := context.Background()
    if err := runtime.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer runtime.Stop(ctx)

    wf := conv3n.NewWorkflow("hello", "Hello World")

    wf.AddNode(&conv3n.Node{
        ID:   "fetch",
        Type: "std/http_request",
        Config: map[string]interface{}{
            "url":    "https://api.github.com/users/octocat",
            "method": "GET",
        },
    })

    wf.AddNode(&conv3n.Node{
        ID:   "transform",
        Type: "std/transform",
        Config: map[string]interface{}{
            "operations": []map[string]interface{}{
                {"type": "pick", "fields": []string{"login", "name", "bio"}},
            },
        },
    })

    wf.AddEdge(&conv3n.Edge{
        ID:     "e1",
        Source: "fetch",
        Target: "transform",
    })

    handle, err := runtime.Execute(ctx, wf, nil)
    if err != nil {
        log.Fatal(err)
    }

    waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    if err := handle.Wait(waitCtx); err != nil {
        log.Fatal(err)
    }

    result, _ := handle.GetNodeResult(ctx, "transform")
    log.Printf("Result: %+v", result)
}
```

### Run It

```bash
go run main.go
```

### What Just Happened?

1. Created a RuntimeV2 instance with default config
2. Built a workflow with 2 nodes: HTTP request → Transform
3. Executed the workflow and waited for completion
4. Retrieved the result from the transform node

### Next Steps

- [Core Concepts](concepts.md) - Understand workflows, nodes, and edges
- [Standard Blocks](blocks.md) - Available built-in blocks
- [Custom Blocks](sdk/blocks.md) - Build your own blocks
- [Event Handling](events.md) - Monitor workflow execution
- [Storage](storage.md) - Execution history and persistence

<div align="center">

---

### Contact

Telegram: [zarazaex](https://t.me/zarazaexe)
<br>
Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io)
<br>
Site: [zarazaex.xyz](https://zarazaex.xyz)

</div>
