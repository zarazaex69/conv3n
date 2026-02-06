# Conv3n Examples

Reference implementations showing how to use the Conv3n engine.

## Server

Full-featured HTTP API server with workflow management, triggers, and execution control.

```bash
go run examples/server/main.go
```

Endpoints:
- `POST /api/run` - Execute workflow
- `POST /api/workflows` - Create workflow
- `GET /api/workflows/{id}` - Get workflow
- `GET /health` - Health check

## CLI Runner

Simple command-line workflow executor.

```bash
go run examples/cli/main.go examples/delay_simple.json
```

## SDK Examples

- `examples/sdk_basic/` - Basic SDK usage
- `examples/sdk_storage/` - Storage operations
- `examples/sdk_events/` - Event handling

## Building Your Own Frontend

Conv3n is designed to be embedded. These examples show different integration patterns:

1. **HTTP API** (server/) - For web frontends
2. **CLI** (cli/) - For terminal tools
3. **Embedded** (sdk_*/) - For Go applications

Your AI-driven frontend would import `github.com/zarazaex69/conv3n/pkg/conv3n` and use the Runtime API directly.
