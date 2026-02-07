GO_CMD  := go
BUN_CMD := bun

.PHONY: all test install deps clean blocks-generate blocks-test test-v2 run-production bench e2e

all: test

# Run all tests
test:
	@echo "[test] Running Go tests..."
	@$(GO_CMD) test ./...
	@echo "[test] Running JS/TS tests..."
	@$(BUN_CMD) run test

# Run coverage tests
cover:
	@echo "[cover] Running coverage tests..."
	@bun test --coverage
	@go test -coverprofile=coverage.out ./internal/...
	@go tool cover -func=coverage.out
	@rm coverage.out
	@echo "[cover] Coverage tests completed successfully"

# Install or check Go and Bun availability
install:
	@echo "[install] Checking Go and Bun..."
	@if ! command -v $(GO_CMD) >/dev/null 2>&1; then \
		echo "[install] Go is not installed or not in PATH."; \
		echo "[install] Please install Go from https://go.dev/dl/"; \
		exit 1; \
	else \
		$(GO_CMD) version; \
	fi
	@if ! command -v $(BUN_CMD) >/dev/null 2>&1; then \
		echo "[install] Bun is not installed or not in PATH."; \
		echo "[install] Installing Bun via official installer..."; \
		curl -fsSL https://bun.sh/install | bash; \
		echo "[install] Restart your shell or source your profile to update PATH."; \
	else \
		$(BUN_CMD) --version; \
	fi

# Install JS/TS dependencies for the web part (optional helper)
deps:
	@echo "[deps] Installing JS/TS dependencies with Bun..."
	@$(BUN_CMD) install
	@echo "[deps] JS/TS dependencies installed successfully"

clean:
	@echo "[clean] Cleaning test artifacts..."
	@rm -f coverage.out
	@echo "[clean] Clean completed successfully"

blocks-generate:
	@echo "[blocks] Generating block manifests..."
	@$(BUN_CMD) run pkg/bunock/cli/generate.ts pkg/blocks
	@echo "[blocks] Manifests generated successfully"

blocks-test:
	@echo "[blocks] Running block tests..."
	@$(BUN_CMD) test pkg/blocks
	@echo "[blocks] Block tests completed successfully"

test-v2:
	@echo "[test-v2] Running V2 tests..."
	@$(GO_CMD) test -v ./internal/engine -run "TestCircuitBreaker|TestRetry"
	@echo "[test-v2] V2 tests completed successfully"

run-production:
	@echo "[production] Starting production example..."
	@$(GO_CMD) run examples/production_ready/main.go

bench:
	@echo "[bench] Running benchmarks..."
	@$(GO_CMD) test -bench=. -benchmem ./internal/engine
	@echo "[bench] Benchmarks completed"

e2e:
	@echo "[e2e] Running end-to-end tests..."
	@$(GO_CMD) test -v -run TestE2E_FullStack -timeout 60s ./internal/api
	@echo "[e2e] E2E tests completed successfully"
