<div align="center">
  <img src="../../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Building Custom Blocks

Create type-safe blocks using the TypeScript SDK.

### Basic Block

```typescript
import { Block } from "#sdk";

interface MyConfig {
    message: string;
    multiplier?: number;
}

interface MyOutput {
    result: string;
    count: number;
}

export class MyBlock extends Block<MyConfig, MyOutput> {
    validate(config: unknown): asserts config is MyConfig {
        if (!config || typeof config !== "object") {
            throw new Error("Config must be an object");
        }

        const c = config as Record<string, unknown>;

        if (typeof c.message !== "string" || c.message.trim() === "") {
            throw new Error("message must be a non-empty string");
        }

        if (c.multiplier !== undefined && typeof c.multiplier !== "number") {
            throw new Error("multiplier must be a number");
        }
    }

    async execute(config: MyConfig, input?: unknown): Promise<MyOutput> {
        const multiplier = config.multiplier || 1;
        const result = config.message.repeat(multiplier);

        return {
            result,
            count: result.length,
        };
    }
}

if (import.meta.main) {
    new MyBlock().run();
}
```

### Schema-Based Validation

```typescript
import { Block, createSchemaValidator, CommonSchemas } from "#sdk";

interface HttpConfig {
    url: string;
    timeout?: number;
}

interface HttpOutput {
    status: number;
    data: unknown;
}

class HttpBlock extends Block<HttpConfig, HttpOutput> {
    private schema = {
        url: CommonSchemas.url,
        timeout: {
            type: 'number' as const,
            default: 5000,
            validate: (v: unknown) => typeof v === 'number' && v > 0,
        }
    };

    validate = createSchemaValidator<HttpConfig>(this.schema);

    async execute(config: HttpConfig): Promise<HttpOutput> {
        const response = await fetch(config.url);
        return {
            status: response.status,
            data: await response.json(),
        };
    }
}
```

### Port Routing

```typescript
import { Block, BlockHelpers } from "#sdk";

interface CheckConfig {
    threshold: number;
}

interface CheckOutput {
    value: number;
    passed: boolean;
}

class CheckBlock extends Block<CheckConfig, CheckOutput> {
    validate(config: unknown): asserts config is CheckConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "threshold", "number");
    }

    async execute(config: CheckConfig, input?: unknown): Promise<CheckOutput> {
        const value = (input as any)?.value || 0;
        const passed = value > config.threshold;

        return { value, passed };
    }

    protected getOutputPort(result: CheckOutput): string {
        return BlockHelpers.getBooleanPort(result.passed);
    }
}
```

### Timeout and Retry

```typescript
import { Block, executeWithTimeoutAndRetry } from "#sdk";

class ResilientBlock extends Block<Config, Output> {
    async execute(config: Config): Promise<Output> {
        return await executeWithTimeoutAndRetry(
            async () => {
                const response = await fetch(config.url);
                return { data: await response.json() };
            },
            5000,
            { attempts: 3, backoff: 'exponential' }
        );
    }
}
```

### Variable Management

```typescript
import { Block } from "#sdk";

class CounterBlock extends Block<Config, Output> {
    async execute(config: Config, input?: unknown): Promise<Output> {
        const inputData = input as { context?: { variables?: Record<string, unknown> } };
        const currentValue = (inputData?.context?.variables?.counter as number) ?? 0;
        const newValue = currentValue + 1;

        this.variables.setGlobal("counter", newValue, 3600);

        return { value: newValue };
    }
}
```

### Block Helpers

```typescript
import { BlockHelpers } from "#sdk";

BlockHelpers.assertObject(config);

BlockHelpers.assertField(config, "url", "string");

BlockHelpers.assertNonEmptyString(config, "name");

const port = BlockHelpers.getHttpPort(response.status);

const port = BlockHelpers.getBooleanPort(condition);

const data = BlockHelpers.parseJSON(text);
```

### Error Handling

```typescript
class SafeBlock extends Block<Config, Output> {
    async execute(config: Config): Promise<Output> {
        try {
            return await this.doWork(config);
        } catch (error) {
            throw new Error(`Operation failed: ${error.message}`);
        }
    }

    protected async handleError(error: unknown): Promise<void> {
        console.error("Block failed:", error);
        await this.writeError(error, "custom_error_port");
    }
}
```

### Block Manifest

Create `block.json` for block registration:

```json
{
    "type": "custom/my_block",
    "name": "My Custom Block",
    "description": "Does something useful",
    "version": "1.0.0",
    "entrypoint": "my_block.ts",
    "config": {
        "message": {
            "type": "string",
            "required": true
        },
        "multiplier": {
            "type": "number",
            "default": 1
        }
    },
    "outputs": {
        "default": "Success output",
        "error": "Error output"
    }
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
