# ROADMAP

## Vision
- Create a pure execution runtime core that can be embedded and controlled by external applications.
- Focus solely on workflow execution speed (Go) and architectural transparency.
- Provide a clean API interface for frontends and external systems to orchestrate workflows.
## Principles
1.  **Runtime only.** Pure execution engine without UI, triggers, or user-facing APIs.
2.  **Embeddable.** Designed to be imported and controlled by other Go applications.
3.  **Observability by default.** All executions are transparent: run history, node results, clear logs.
4.  **Zero dependencies on external services.** Self-contained execution core.
---

## Phase 0. Engineering Foundations
- [X] **CI/CD**: gofmt, golangci-lint, runtime smoke tests.
- [X] **Packaging**: Makefile targets for build/test as Go library.
- [ ] **Documentation**: Go package docs, embedding guide, execution model description.

## Phase 1. Runtime Core
1.  **Graph Engine**
    - [X] Pointer-based execution with support for loops, branches, multiple start nodes.
    - [X] Variable management: set/get var, scoping, input data templating.
2.  **Block Lifecycle**
    - [X] Unified input/output protocol (stdin/stdout JSON) for Bun blocks.
    - [X] Extended STD set: HTTP, Condition, Delay, Loop, Transform, File, Database, Custom Code.
    - [X] Basic runtime in `pkg/bunock` (stdin/stdout JSON, output structure validation).
    - [X] Type-safe SDK helpers for blocks on top of `pkg/bunock` (Block class, BlockHelpers).
3.  **Reliability**
    - [X] Timeouts for each node's execution (config.timeout_ms + default).
    - [X] Resume: save ExecutionContext and restart from the last node.
    - [X] Idempotency: agreements on de-duplication keys (nodeID + executionID).
4.  **Storage**
    - [X] Storage interface + SQLite implementation with run history and node results.

## Phase 2. Core API Interface
1.  **Execution Control**
    - [X] Programmatic workflow execution interface (`Execute(workflow, input)`)
    - [X] Execution lifecycle management: stop, pause, resume operations
    - [X] Execution status tracking and result retrieval
    - [ ] Batch execution capabilities
2.  **Concurrency Management**
    - [X] Configurable worker pools (WorkerPool)
    - [X] Graceful shutdown for long-running tasks (ExecutionRegistry, context cancellation)
3.  **Event System**
    - [X] Execution event callbacks (start, complete, error, node execution)
    - [X] Progress reporting interface for external monitoring

## Phase 3. Library Interface
1.  **Go Package API**
    - [x] Clean public interface for embedding (`runtime.New()`, `runtime.Execute()`)
    - [x] Configuration struct for runtime behavior
    - [x] Thread-safe operations for concurrent usage
2.  **Block Development SDK**
    - [X] Block development framework and testing utilities
    - [X] Block registration and discovery system
    - [X] Custom block development guide
---

## Final State
-   **Pure runtime core**: Embeddable Go library for workflow execution
-   **Frontend agnostic**: Can be controlled by web UIs, CLIs, or other applications
-   **Production-grade**: Reliable execution, observability, scalability within the core
-   **Library first**: Designed as a dependency, not a standalone application
