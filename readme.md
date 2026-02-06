<div align="center">
  <img src="assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## About

Conv3n is an embeddable workflow automation engine for Go applications.

**Not a standalone tool** — it's a library you import into your own projects.

## Installation

```bash
go get github.com/zarazaex69/conv3n/pkg/conv3n
```

## Quick Start

```go
package main

import (
    "context"
    "github.com/zarazaex69/conv3n/pkg/conv3n"
)

func main() {
    runtime, _ := conv3n.New(conv3n.DefaultConfig())
    runtime.Start(context.Background())
    defer runtime.Close()
    
    wf := conv3n.NewWorkflow("wf_1", "My Workflow")
    wf.AddNode(&conv3n.Node{
        ID:   "fetch",
        Type: "std/http_request",
        Config: map[string]interface{}{
            "url": "https://api.example.com/data",
        },
    })
    
    handle, _ := runtime.Execute(context.Background(), wf, nil)
    handle.Wait(context.Background())
}
```

## Examples

See `examples/` directory:
- `reference_server.go` — Full HTTP API server
- `cli_runner.go` — Simple CLI workflow runner
- `sdk_basic.go` — SDK usage examples

## Tech Stack

- **Engine**: Go 1.25+ with SQLite
- **Block Runtime**: Bun for TypeScript/JavaScript blocks
- **SDK**: TypeScript SDK for custom block development


<div align="center">

---

### Contact

Telegram: [zarazaex](https://t.me/zarazaexe)
<br>
Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io)
<br>
Site: [zarazaex.xyz](https://zarazaex.xyz)

</div>
