APP_NAME := conv3n
GO_CMD  := go
BUN_CMD := bun

BIN_DIR := bin

.PHONY: all build test install deps clean blocks-generate blocks-test

all: build

build:
	@echo "[build] Building $(APP_NAME) for current platform..."
	@mkdir -p $(BIN_DIR)
	@$(GO_CMD) build -o $(BIN_DIR)/$(APP_NAME) ./cmd/conv3n
	@echo "$(BIN_DIR)/$(APP_NAME) built successfully"

# Run all tests
test:
	@echo "[test] Running Go tests..."
	@$(GO_CMD) test ./...
	@echo "[test] Running JS/TS tests..."
	@$(BUN_CMD) run test
	@echo "[test] All tests completed successfully"

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
	@echo "[clean] Removing build artifacts..."
	@rm -rf $(BIN_DIR)
	@echo "[clean] Build artifacts removed successfully"

blocks-generate:
	@echo "[blocks] Generating block manifests..."
	@$(BUN_CMD) run pkg/bunock/cli/generate.ts pkg/blocks
	@echo "[blocks] Manifests generated successfully"

blocks-test:
	@echo "[blocks] Running block tests..."
	@$(BUN_CMD) test pkg/blocks
	@echo "[blocks] Block tests completed successfully"
