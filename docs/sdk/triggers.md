<div align="center">
  <img src="../../assets/logo.png" alt="Conv3n" width="400"/>
</div>

<div align="center">

![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![TypeScript](https://img.shields.io/badge/-TypeScript-0D1117?style=flat-square&logo=typescript&logoColor=377CC8)
![Bun](https://img.shields.io/badge/-Bun-0D1117?style=flat-square&logo=Bun&logoColor=F3E6D8)
![SQLite](https://img.shields.io/badge/-SQLite-0D1117?style=flat-square&logo=sqlite&logoColor=003B57)

</div>

## Building Custom Triggers

Create event-driven triggers using the TypeScript SDK.

### Basic Trigger

```typescript
import { Trigger, type TriggerContext, BlockHelpers } from "#sdk";

interface MyTriggerConfig extends Record<string, unknown> {
    interval: number;
}

class MyTrigger extends Trigger<MyTriggerConfig> {
    private intervalId: Timer | null = null;

    validate(config: unknown): asserts config is MyTriggerConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "interval", "number");

        const interval = config.interval as number;
        if (interval < 100) {
            throw new Error("Interval must be at least 100ms");
        }
    }

    async start(ctx: TriggerContext<MyTriggerConfig>): Promise<void> {
        const { interval } = ctx.config;

        this.intervalId = setInterval(async () => {
            await ctx.fire({
                timestamp: Date.now(),
                message: "Interval triggered"
            });
        }, interval);

        console.log(`Trigger started with interval ${interval}ms`);
    }

    async stop(ctx: TriggerContext<MyTriggerConfig>): Promise<void> {
        if (this.intervalId) {
            clearInterval(this.intervalId);
            this.intervalId = null;
            console.log("Trigger stopped");
        }
    }
}

if (import.meta.main) {
    new MyTrigger().run();
}
```

### HTTP Server Trigger

```typescript
import { Trigger, type TriggerContext, BlockHelpers } from "#sdk";

interface HttpConfig extends Record<string, unknown> {
    port: number;
    path?: string;
}

class HttpServerTrigger extends Trigger<HttpConfig> {
    private server: ReturnType<typeof Bun.serve> | null = null;

    validate(config: unknown): asserts config is HttpConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "port", "number");
    }

    async start(ctx: TriggerContext<HttpConfig>): Promise<void> {
        const { port, path = "/" } = ctx.config;

        this.server = Bun.serve({
            port,
            async fetch(req) {
                const url = new URL(req.url);

                if (url.pathname !== path) {
                    return new Response("Not Found", { status: 404 });
                }

                const body = await req.json();

                try {
                    const result = await ctx.fire(body);
                    return new Response(JSON.stringify(result), {
                        status: 200,
                        headers: { "Content-Type": "application/json" }
                    });
                } catch (error) {
                    return new Response(
                        JSON.stringify({ error: error.message }),
                        { status: 500 }
                    );
                }
            }
        });

        console.log(`HTTP server started on port ${port}`);
    }

    async stop(ctx: TriggerContext<HttpConfig>): Promise<void> {
        if (this.server) {
            this.server.stop();
            this.server = null;
        }
    }
}
```

### File Watcher Trigger

```typescript
import { Trigger, type TriggerContext, BlockHelpers } from "#sdk";
import { watch } from "fs";

interface FileWatchConfig extends Record<string, unknown> {
    path: string;
    events?: string[];
}

class FileWatchTrigger extends Trigger<FileWatchConfig> {
    private watcher: any = null;

    validate(config: unknown): asserts config is FileWatchConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "path");
    }

    async start(ctx: TriggerContext<FileWatchConfig>): Promise<void> {
        const { path, events = ["change", "rename"] } = ctx.config;

        this.watcher = watch(path, async (eventType, filename) => {
            if (events.includes(eventType)) {
                await ctx.fire({
                    eventType,
                    filename,
                    path,
                    timestamp: Date.now()
                });
            }
        });

        console.log(`Watching ${path} for events: ${events.join(", ")}`);
    }

    async stop(ctx: TriggerContext<FileWatchConfig>): Promise<void> {
        if (this.watcher) {
            this.watcher.close();
            this.watcher = null;
        }
    }
}
```

### Message Handler

```typescript
class WebhookTrigger extends Trigger<Config> {
    async start(ctx: TriggerContext<Config>): Promise<void> {
        this.registerMessageHandler("invoke", async (message) => {
            console.log("Webhook invoked");
            await ctx.fire(message.payload);
        });

        console.log("Webhook trigger ready");
    }

    async stop(ctx: TriggerContext<Config>): Promise<void> {
        console.log("Webhook trigger stopped");
    }
}
```

### Trigger Context

```typescript
interface TriggerContext<TConfig> {
    config: TConfig;
    fire: <TPayload = unknown>(payload: TPayload) => Promise<unknown>;
}
```

The `fire` method starts a workflow execution with the provided payload.

### Trigger Lifecycle

1. **Validate**: Config validation on trigger creation
2. **Start**: Initialize resources (servers, timers, watchers)
3. **Fire**: Trigger workflow executions
4. **Stop**: Cleanup resources on shutdown

<div align="center">

---

### Contact

Telegram: [zarazaex](https://t.me/zarazaexe)
<br>
Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io)
<br>
Site: [zarazaex.xyz](https://zarazaex.xyz)

</div>
