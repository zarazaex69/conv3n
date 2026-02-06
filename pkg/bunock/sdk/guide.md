# Block SDK Guide

## Fast Start

### 1. Create Block File

```typescript
import { Block } from "../../bunock/sdk/sdk.ts";

interface MyConfig {
    url: string;
    timeout?: number;
}

interface MyOutput {
    status: number;
    body: string;
}

export class MyBlock extends Block<MyConfig, MyOutput> {
    validate(config: unknown): asserts config is MyConfig {
        if (!config || typeof config !== "object") {
            throw new Error("Config must be an object");
        }
        const c = config as Record<string, unknown>;
        if (typeof c.url !== "string") {
            throw new Error("url is required");
        }
    }

    async execute(config: MyConfig, input?: unknown): Promise<MyOutput> {
        const response = await fetch(config.url, {
            signal: AbortSignal.timeout(config.timeout || 5000),
        });
        return {
            status: response.status,
            body: await response.text(),
        };
    }
}

if (import.meta.main) {
    new MyBlock().run();
}
```

### 2. Create Manifest

**Option A: Manual** (`my_block.block.json`)

```json
{
  "name": "custom/my_block",
  "type": "custom/my_block",
  "description": "Fetches data from URL",
  "scriptPath": "my_block.ts",
  "category": "custom",
  "inputs": [{ "name": "main" }],
  "outputs": [{ "name": "default" }]
}
```

**Option B: Auto-generate** (add JSDoc annotations)

```typescript
/**
 * @block custom/my_block
 * @description Fetches data from URL
 * @category custom
 * @inputs [{"name":"main"}]
 * @outputs [{"name":"default"}]
 */
export class MyBlock extends Block<MyConfig, MyOutput> {
```

Then run: `bun run pkg/bunock/cli/generate.ts pkg/blocks`

### 3. Register in Go

```go
runner := engine.NewBunRunner("pkg/blocks")
if err := runner.LoadBlocks(); err != nil {
    log.Fatal(err)
}
```

## Advanced: Multi-Port Routing

```typescript
export class ConditionalBlock extends Block<CondConfig, CondOutput> {
    protected getOutputPort(result: CondOutput): string {
        return result.condition ? "true" : "false";
    }
}
```

Manifest:

```json
{
  "outputs": [
    { "name": "true", "description": "Condition passed" },
    { "name": "false", "description": "Condition failed" }
  ]
}
```

## Testing

```typescript
import { test, expect } from "bun:test";

test("MyBlock executes", async () => {
    const block = new MyBlock();
    const result = await block.execute({ url: "https://example.com" });
    expect(result.status).toBe(200);
});
```

## CLI Commands

```bash
bun run pkg/bunock/cli/generate.ts pkg/blocks

bun test pkg/blocks/custom/my_block.test.ts

bun run pkg/blocks/custom/my_block.ts < input.json
```

## Architecture

```
pkg/blocks/
├── custom/
│   ├── my_block.ts           # Implementation
│   ├── my_block.block.json   # Manifest (auto-generated or manual)
│   └── my_block.test.ts      # Tests
└── std/
    └── http_request.ts
```

Go engine scans `**/*.block.json` at startup and registers blocks dynamically.
