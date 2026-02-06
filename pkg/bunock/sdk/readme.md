<div align="center">
  <img src="../../../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

Type-safe SDK for building workflow blocks in the CONV3N engine.

## Overview

The Block SDK provides a structured, type-safe way to create custom blocks for the CONV3N workflow engine. It handles the stdin/stdout communication protocol automatically, allowing you to focus on implementing your block's core logic.

## Fast Start

### Basic Block Example

```typescript
import { Block, BlockHelpers } from "../../bunock/sdk/sdk.ts";

// Define your configuration type
interface MyBlockConfig {
    message: string;
    count: number;
}

// Define your output type
interface MyBlockOutput {
    result: string;
}

// Implement the block
class MyBlock extends Block<MyBlockConfig, MyBlockOutput> {
    // Validate configuration
    validate(config: unknown): asserts config is MyBlockConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "message");
        BlockHelpers.assertField(config, "count", "number");
    }

    // Execute block logic
    async execute(config: MyBlockConfig, input?: unknown): Promise<MyBlockOutput> {
        const result = config.message.repeat(config.count);
        return { result };
    }
}

// Run the block if this is the entry point
if (import.meta.main) {
    new MyBlock().run();
}
```

### HTTP Request Block Example

```typescript
import { Block, BlockHelpers } from "../../bunock/sdk/sdk.ts";

interface HttpConfig {
    url: string;
    method?: string;
}

interface HttpOutput {
    status: number;
    data: unknown;
}

class HttpBlock extends Block<HttpConfig, HttpOutput> {
    validate(config: unknown): asserts config is HttpConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "url");
    }

    async execute(config: HttpConfig): Promise<HttpOutput> {
        const response = await fetch(config.url, {
            method: config.method || "GET",
        });
        
        const data = await response.text();
        
        return {
            status: response.status,
            data: BlockHelpers.parseJSON(data),
        };
    }

    // Override to implement custom routing based on HTTP status
    protected getOutputPort(result: HttpOutput): string {
        return BlockHelpers.getHttpPort(result.status);
    }
}

if (import.meta.main) {
    new HttpBlock().run();
}
```

### Condition Block Example

```typescript
import { Block, BlockHelpers } from "../../bunock/sdk/sdk.ts";

interface ConditionConfig {
    expression: string;
}

interface ConditionOutput {
    result: boolean;
}

class ConditionBlock extends Block<ConditionConfig, ConditionOutput> {
    validate(config: unknown): asserts config is ConditionConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "expression");
    }

    async execute(config: ConditionConfig, input?: unknown): Promise<ConditionOutput> {
        // Evaluate expression (simplified example)
        const result = eval(config.expression);
        return { result: Boolean(result) };
    }

    // Route based on boolean result
    protected getOutputPort(result: ConditionOutput): string {
        return BlockHelpers.getBooleanPort(result.result);
    }
}

if (import.meta.main) {
    new ConditionBlock().run();
}
```

## New SDK Features

### Import Alias

Use the convenient `#sdk` alias instead of relative paths:

```typescript
// Old way
import { Block, BlockHelpers } from "../../bunock/sdk/sdk.ts";

// New way
import { Block, BlockHelpers } from "#sdk";
```

### Schema-Based Validation

Simplify configuration validation with declarative schemas:

```typescript
import { Block, createSchemaValidator, CommonSchemas } from "#sdk";

interface MyConfig {
    url: string;
    timeout: number;
    retries: number;
}

class MyBlock extends Block<MyConfig, MyOutput> {
    validate = createSchemaValidator<MyConfig>({
        url: CommonSchemas.url,
        timeout: { type: 'number', default: 5000 },
        retries: { 
            type: 'number', 
            default: 3,
            validate: (v) => v >= 1 && v <= 10,
            errorMessage: 'Retries must be between 1 and 10'
        }
    });
    
    async execute(config: MyConfig): Promise<MyOutput> {
        // config.timeout will be 5000 if not provided
        // ...
    }
}
```

### Timeout and Retry Decorators

Add timeout and retry logic to async operations:

```typescript
import { Block, executeWithTimeoutAndRetry } from "#sdk";

class HttpBlock extends Block<HttpConfig, HttpOutput> {
    async execute(config: HttpConfig): Promise<HttpOutput> {
        // Automatically retry up to 3 times with exponential backoff
        // Each attempt has a 5 second timeout
        return await executeWithTimeoutAndRetry(
            async () => {
                const response = await fetch(config.url);
                return { status: response.status, data: await response.json() };
            },
            5000, // timeout per attempt
            { attempts: 3, backoff: 'exponential' }
        );
    }
}
```

## Core Concepts

### Block Lifecycle

1. **Input Reading**: SDK reads JSON from stdin
2. **Validation**: Your `validate()` method checks configuration
3. **Execution**: Your `execute()` method runs the block logic
4. **Output Writing**: SDK writes JSON to stdout with routing port

### Configuration Validation

Use `BlockHelpers` for common validation patterns:

```typescript
validate(config: unknown): asserts config is MyConfig {
    // Ensure config is an object
    BlockHelpers.assertObject(config);
    
    // Check required string field
    BlockHelpers.assertNonEmptyString(config, "url");
    
    // Check required typed field
    BlockHelpers.assertField(config, "timeout", "number");
    
    // Check optional field
    if ("headers" in config) {
        BlockHelpers.assertField(config, "headers", "object");
    }
}
```

### Output Routing

Blocks can route execution to different ports based on results:

```typescript
protected getOutputPort(result: MyOutput): string {
    // HTTP status-based routing
    return BlockHelpers.getHttpPort(result.status);
    
    // Boolean-based routing
    return BlockHelpers.getBooleanPort(result.success);
    
    // Custom routing
    if (result.error) return "error";
    if (result.retry) return "retry";
    return "default";
}
```

### Error Handling

The SDK handles errors automatically, but you can customize:

```typescript
protected async handleError(error: unknown): Promise<void> {
    // Log error for debugging
    console.error("Block failed:", error);
    
    // Write error to stdout with custom port
    await this.writeError(error, "custom_error_port");
}
```

## API Reference

### Block Class

Abstract base class for all blocks.

#### Methods to Implement

- `validate(config: unknown): asserts config is TConfig`
  - Validate and type-narrow the configuration
  - Throw error if validation fails

- `execute(config: TConfig, input?: unknown): Promise<TOutput>`
  - Implement your block's core logic
  - Return output data

#### Methods to Override (Optional)

- `getOutputPort(result: TOutput): string`
  - Determine routing port based on result
  - Default: returns "default"

- `handleError(error: unknown): Promise<void>`
  - Custom error handling
  - Default: writes error to stdout and exits

- `readInput(): Promise<BlockInput<TConfig>>`
  - Custom input parsing
  - Default: reads JSON from stdin

#### Protected Utilities

- `writeOutput(data: TOutput, port: string): Promise<void>`
  - Write success result to stdout

- `writeError(error: unknown, port?: string): Promise<void>`
  - Write error result to stdout and exit

- `formatError(error: unknown): ErrorDetails`
  - Convert error to standard format

### BlockHelpers Class

Static utility methods for common operations.

#### Result Creation

- `createSuccessResult<T>(data: T, port?: string): BlockOutput<T>`
- `createErrorResult(message: string, type?: string, stack?: string): BlockOutput`

#### Validation

- `assertObject(value: unknown, fieldName?: string): asserts value is Record<string, unknown>`
- `assertField<T>(obj: Record<string, unknown>, field: string, type: string): asserts obj is Record<string, T>`
- `assertNonEmptyString(obj: Record<string, unknown>, field: string): void`

#### Utilities

- `parseJSON(text: string): unknown` - Safe JSON parsing with fallback
- `getHttpPort(status: number): string` - HTTP status-based routing
- `getBooleanPort(value: boolean): string` - Boolean-based routing

## Testing Your Block

```typescript
import { test, expect } from "bun:test";
import { MyBlock } from "./my_block.ts";

test("MyBlock executes successfully", async () => {
    const block = new MyBlock();
    
    const config = { message: "test", count: 3 };
    const result = await block.execute(config);
    
    expect(result.result).toBe("testtesttest");
});

test("MyBlock validates config", () => {
    const block = new MyBlock();
    
    expect(() => {
        block.validate({});
    }).toThrow("Missing required field: message");
});
```

## Best Practices

1. **Type Safety**: Always define strict TypeScript types for config and output
2. **Validation**: Validate all required fields in `validate()` method
3. **Error Messages**: Provide clear, actionable error messages
4. **Idempotency**: Make blocks idempotent when possible
5. **Timeouts**: Handle long-running operations with timeouts
6. **Testing**: Write unit tests for validation and execution logic

## Migration from Raw Blocks

If you have existing blocks using raw stdin/stdout:

**Before:**
```typescript
const input = await Bun.stdin.json();
if (!input.config.url) throw new Error("Missing url");
const result = await fetch(input.config.url);
await Bun.write(Bun.stdout, JSON.stringify({ data: result, port: "default" }));
```

**After:**
```typescript
class MyBlock extends Block<MyConfig, MyOutput> {
    validate(config: unknown): asserts config is MyConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "url");
    }
    
    async execute(config: MyConfig): Promise<MyOutput> {
        return await fetch(config.url);
    }
}

if (import.meta.main) {
    new MyBlock().run();
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
