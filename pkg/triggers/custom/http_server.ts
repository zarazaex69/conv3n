import { Trigger, type TriggerContext, BlockHelpers } from "#sdk";

interface HttpServerConfig extends Record<string, unknown> {
    port: number;
    path?: string;
}

interface HttpRequestPayload {
    method: string;
    url: string;
    headers: Record<string, string>;
    body?: unknown;
    timestamp: number;
}

class HttpServerTrigger extends Trigger<HttpServerConfig> {
    private server: ReturnType<typeof Bun.serve> | null = null;

    validate(config: unknown): asserts config is HttpServerConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "port", "number");

        const port = config.port as number;
        if (port < 1 || port > 65535) {
            throw new Error("Port must be between 1 and 65535");
        }
    }

    async start(ctx: TriggerContext<HttpServerConfig>): Promise<void> {
        const { port, path = "/" } = ctx.config;

        this.server = Bun.serve({
            port,
            async fetch(req) {
                const url = new URL(req.url);

                if (url.pathname !== path) {
                    return new Response("Not Found", { status: 404 });
                }

                let body: unknown = undefined;
                const contentType = req.headers.get("content-type");

                if (contentType?.includes("application/json")) {
                    try {
                        body = await req.json();
                    } catch {
                        body = await req.text();
                    }
                } else if (req.method !== "GET" && req.method !== "HEAD") {
                    body = await req.text();
                }

                const payload: HttpRequestPayload = {
                    method: req.method,
                    url: url.pathname + url.search,
                    headers: Object.fromEntries(req.headers.entries()),
                    body,
                    timestamp: Date.now()
                };

                try {
                    const result = await ctx.fire(payload);
                    return new Response(JSON.stringify(result), {
                        status: 200,
                        headers: { "Content-Type": "application/json" }
                    });
                } catch (error) {
                    return new Response(
                        JSON.stringify({
                            error: error instanceof Error ? error.message : String(error)
                        }),
                        {
                            status: 500,
                            headers: { "Content-Type": "application/json" }
                        }
                    );
                }
            }
        });

        console.log(`HTTP server trigger started on port ${port}, path ${path}`);
    }

    async stop(ctx: TriggerContext<HttpServerConfig>): Promise<void> {
        if (this.server) {
            this.server.stop();
            this.server = null;
            console.log(`HTTP server trigger stopped on port ${ctx.config.port}`);
        }
    }
}

if (import.meta.main) {
    new HttpServerTrigger().run();
}
