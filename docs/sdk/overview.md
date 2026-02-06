<div align="center">
  <img src="../../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## TypeScript SDK

Build custom blocks and triggers using the Conv3n TypeScript SDK.

### Installation

The SDK is included in the Conv3n repository:

```typescript
import { Block, Trigger } from "#sdk";
```

### Core Classes

**Block:**
```typescript
abstract class Block<TConfig, TOutput> {
    abstract validate(config: unknown): asserts config is TConfig;
    abstract execute(config: TConfig, input?: unknown): Promise<TOutput>;
    protected getOutputPort(result: TOutput): string;
}
```

**Trigger:**
```typescript
abstract class Trigger<TConfig> {
    abstract validate(config: unknown): asserts config is TConfig;
    abstract start(ctx: TriggerContext<TConfig>): Promise<void>;
    abstract stop(ctx: TriggerContext<TConfig>): Promise<void>;
}
```

### Utilities

**Schema Validation:**
```typescript
import { createSchemaValidator, CommonSchemas } from "#sdk";

const schema = {
    url: CommonSchemas.url,
    port: CommonSchemas.port,
    email: CommonSchemas.email,
};

validate = createSchemaValidator<Config>(schema);
```

**Decorators:**
```typescript
import { withTimeout, withRetry, executeWithTimeoutAndRetry } from "#sdk";

const result = await withTimeout(promise, { ms: 5000 });

const result = await withRetry(fn, { 
    attempts: 3, 
    backoff: 'exponential' 
});

const result = await executeWithTimeoutAndRetry(fn, 5000, { attempts: 3 });
```

**Variables:**
```typescript
import { VariableManager } from "#sdk";

const variables = new VariableManager();

variables.setGlobal("key", "value", 3600);
variables.setWorkflow("counter", 0);
variables.setExecution("temp", data);

const commands = variables.getCommands();
```

**Helpers:**
```typescript
import { BlockHelpers } from "#sdk";

BlockHelpers.assertObject(config);
BlockHelpers.assertField(config, "url", "string");
BlockHelpers.assertNonEmptyString(config, "name");

const port = BlockHelpers.getHttpPort(status);
const port = BlockHelpers.getBooleanPort(condition);
const data = BlockHelpers.parseJSON(text);
```

### Project Structure

```
pkg/blocks/
├── std/
│   ├── http_request.ts
│   ├── transform.ts
│   └── ...
└── custom/
    ├── my_block.ts
    └── my_block.block.json

pkg/triggers/
├── std/
│   ├── cron.ts
│   └── webhook.ts
└── custom/
    ├── my_trigger.ts
    └── my_trigger.trigger.json
```

### Block Manifest

```json
{
    "type": "custom/my_block",
    "name": "My Block",
    "description": "Custom block",
    "version": "1.0.0",
    "entrypoint": "my_block.ts",
    "config": {
        "field": {
            "type": "string",
            "required": true
        }
    },
    "outputs": {
        "default": "Success",
        "error": "Error"
    }
}
```

### Testing

```typescript
import { describe, test, expect } from "bun:test";

describe("MyBlock", () => {
    test("validates config", () => {
        const block = new MyBlock();
        expect(() => block.validate({})).toThrow();
    });

    test("executes successfully", async () => {
        const block = new MyBlock();
        const result = await block.execute({ message: "test" });
        expect(result.result).toBe("test");
    });
});
```

Run tests:
```bash
bun test pkg/blocks/custom/my_block.test.ts
```

### Next Steps

- [Building Blocks](blocks.md) - Create custom blocks
- [Building Triggers](triggers.md) - Create custom triggers
- [Examples](examples.md) - Real-world examples

<div align="center">

---

### Contact

Telegram: [zarazaex](https://t.me/zarazaexe)
<br>
Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io)
<br>
Site: [zarazaex.xyz](https://zarazaex.xyz)

</div>
