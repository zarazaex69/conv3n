# ROADMAP

## Vision
- Create an open, completely free no-code tool that combines the best ideas from n8n, Scratch, and custom runtimes.
- Emphasize execution speed (Go + Bun), architectural transparency, and ease of local installation.
- Develop only the necessary level of security: responsibility for isolation lies with the platform deployer.

## Principles
1.  **Runtime first.** First, perfect the execution core, then build out APIs and interfaces.
2.  **Easy extensibility.** Any block or integration should be added via a clear SDK without hidden limitations.
3.  **Observability by default.** All executions are transparent: run history, node results, clear logs.
4.  **Zero paywall.** BSD license, no paid features or donations.

---

## Phase 0. Engineering Foundations
- [X] **CI/CD**: gofmt, golangci-lint, bun test, runtime smoke tests.
- [X] **Packaging**: Makefile targets for build/test, scripts for local deployment (Docker/Bun/Go).
- [ ] **Documentation**: README, developer guide, block structure description.

## Phase 1. Runtime Core 2.0
1.  **Graph Engine**
    - [X] Pointer-based execution with support for loops, branches, multiple start nodes.
    - [X] Variable management: set/get var, scoping, input data templating.
2.  **Block Lifecycle**
    - [X] Unified input/output protocol (stdin/stdout JSON) for Bun blocks.
    - [X] Extended STD set: HTTP, Condition, Delay, Loop, Transform, File, Database, Webhook, Custom Code.
    - [X] Basic runtime and test runner in `pkg/bunock` (stdin/stdout JSON, output structure validation).
    - [X] Type-safe SDK helpers for blocks on top of `pkg/bunock` (Block class, BlockHelpers, 49 unit tests).
3.  **Reliability**
    - [X] Timeouts for each node's execution (config.timeout_ms + default).
    - [ ] Heartbeat metrics per node.
    - [X] Resume: save ExecutionContext and restart from the last node.
    - [X] Idempotency: agreements on de-duplication keys (nodeID + executionID).
4.  **Storage**
    - [X] Storage interface + SQLite implementation with run history and node results.
    [ ] Alternative backends (Postgres, etc.) through the same abstraction.

## Phase 2. Triggers and Integrations
1.  **Trigger Engine**
    - [X] Basic architecture: TriggerManager, TriggerRunner interface.
    - [X] Cron triggers (robfig/cron/v3, asynchronous execution).
    - [X] Interval triggers (time.Ticker, graceful shutdown).
    - [X] Webhook triggers with signature verification and retry queue.
    - [ ] Event triggers (files, messages, external events via IPC).
    - [X] Storage for triggers (triggers, trigger_executions tables).
    - [X] API endpoints for CRUD triggers.
2.  **Concurrency Limiting**
    - [X] Configurable worker pools per workflow / globally (WorkerPool).
    - [X] Kill-switch and graceful shutdown for long-running tasks (ExecutionRegistry, context cancellation).
3.  **Queues and Retries**
    - [ ] Backoff policies, DLQ for failed tasks.
    - [ ] Tags for re-running a specific node/chain.
4.  **Integration SDKs**
    - [ ] Templates for REST/SOAP, databases, queues (RabbitMQ, Kafka), file systems.

## Phase 3. API and DevOps Layer
1.  **Public API (REST/gRPC)**
    - [X] Workflow CRUD (create, read, update, delete via HTTP API).
    - [X] One-time workflow execution via HTTP (`POST /api/run`).
    - [X] Execution lifecycle management: stop (`POST /api/executions/:id/stop`), restart, batch operations.
    - [X] Get execution history and node results via HTTP API.
    - [X] ExecutionStatusCancelled for tracking stopped executions.
    - [ ] Trigger statuses and metrics via API.
2.  **Auth & Multitenancy (minimal)**
    - [ ] Tokens/keys per workspace.
    - [ ] RBAC at workflow and secrets level.
3.  **Secrets & Config**
    - [ ] Storage for environment variables and secrets (Vault-compatible interface).
    - [ ] Link to blocks via variables.
4.  **DevOps tooling**
    - [ ] Observability endpoints (Prometheus metrics, structured logs).
    - [ ] CLI utility for migrations, workflow import/export, backups.

## Phase 4. Studio & UX (no-code environment)
1.  **Visual Builder**
    - [ ] Drag-n-drop canvas with live validation.
    - [ ] Config inspector, live preview of node results.
2.  **Debugger & Playback**
    - [ ] Step-by-step execution playback, context viewing.
    - [ ] Breakpoints and conditional stops on nodes.
3.  **Template Gallery**
    - [ ] Set of ready-made workflows (API proxy, ETL, cron jobs, AI pipelines).
4.  **Education Layer**
    - [ ] Interactive "Scratch for adults" lessons: blocks as constructors, hints.

## Phase 5. Ecosystem and Community
1.  **Marketplace (optional offline)**
    - [ ] Repository of blocks/triggers (git-based), manual moderation.
    - [ ] Versioning and block compatibility.
2.  **Community Tooling**
    - [ ] Documentation for creating blocks, best practices.
    - [ ] Project showcase, examples of production use.
3.  **Sustainability**
    - [ ] RFC process, roadmap review every 6 months.
    - [ ] Release train policy (LTS/edge versions).

---

## Final State
-   **Best no-code runtime**: Go speed, Bun flexibility, full openness.
-   **n8n + Scratch experience**: visual builder, rich set of blocks, support for learning and experimentation.
-   **Production-grade**: triggers, API, observability, retries, scalability.
-   **0 monetization**: all functionality available for free under BSD, no donations accepted.
