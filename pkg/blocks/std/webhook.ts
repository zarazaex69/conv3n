import { Block, BlockHelpers } from "#sdk";

const MAX_TIMEOUT_MS = 30000;
const DEFAULT_TIMEOUT_MS = 30000;

interface WebhookConfig {
    url: string;
    method: "POST" | "PUT" | "PATCH";
    headers?: Record<string, string>;
    body?: unknown;
    timeout?: number;
}

interface WebhookOutput {
    status: number;
    statusText: string;
    headers: Record<string, string>;
    data: unknown;
    duration: number;
}

export class WebhookBlock extends Block<WebhookConfig, WebhookOutput> {
    validate(config: unknown): asserts config is WebhookConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "url");

        try {
            new URL(config.url as string);
        } catch {
            throw new Error(`Invalid URL format: ${config.url}`);
        }

        BlockHelpers.assertField(config, "method", "string");
        const validMethods = ["POST", "PUT", "PATCH"];
        if (!validMethods.includes(config.method as string)) {
            throw new Error(`Invalid HTTP method: ${config.method}. Must be one of: ${validMethods.join(", ")}`);
        }

        if ("headers" in config && config.headers !== undefined) {
            BlockHelpers.assertField(config, "headers", "object");
        }

        if ("timeout" in config && config.timeout !== undefined) {
            BlockHelpers.assertField(config, "timeout", "number");
            const timeout = config.timeout as number;
            if (timeout <= 0) {
                throw new Error("timeout must be a positive number");
            }
            if (timeout > MAX_TIMEOUT_MS) {
                throw new Error(`Timeout (${timeout}ms) exceeds maximum allowed (${MAX_TIMEOUT_MS}ms)`);
            }
        }
    }

    async execute(config: WebhookConfig): Promise<WebhookOutput> {
        const timeout = config.timeout || DEFAULT_TIMEOUT_MS;
        const startTime = Date.now();

        let requestBody: string | undefined;
        const headers = config.headers || {};

        if (config.body !== undefined) {
            if (typeof config.body === "object") {
                requestBody = JSON.stringify(config.body);
                if (!headers["Content-Type"] && !headers["content-type"]) {
                    headers["Content-Type"] = "application/json";
                }
            } else {
                requestBody = String(config.body);
            }
        }

        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), timeout);

        try {
            const response = await fetch(config.url, {
                method: config.method,
                headers,
                body: requestBody,
                signal: controller.signal,
            });

            clearTimeout(timeoutId);

            const duration = Date.now() - startTime;
            const responseText = await response.text();

            let parsedData: unknown;
            try {
                parsedData = JSON.parse(responseText);
            } catch {
                parsedData = responseText;
            }

            if (!response.ok) {
                throw new Error(
                    `HTTP ${response.status} ${response.statusText}: ${typeof parsedData === "string" ? parsedData : JSON.stringify(parsedData)}`
                );
            }

            return {
                status: response.status,
                statusText: response.statusText,
                headers: Object.fromEntries(response.headers.entries()),
                data: parsedData,
                duration,
            };
        } catch (error) {
            clearTimeout(timeoutId);

            if ((error as Error).name === "AbortError") {
                throw new Error(`Request timeout after ${timeout}ms`);
            }

            if ((error as Error).message.includes("fetch failed")) {
                throw new Error(`Network error: ${(error as Error).message}`);
            }

            throw error;
        }
    }
}

if (import.meta.main) {
    new WebhookBlock().run();
}
