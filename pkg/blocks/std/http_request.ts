import { Block, BlockHelpers } from "#sdk";

interface HttpRequestConfig {
    url: string;
    method?: string;
    headers?: Record<string, string>;
    body?: unknown;
}

interface HttpRequestOutput {
    status: number;
    statusText: string;
    headers: Record<string, string>;
    data: unknown;
}

export class HttpRequestBlock extends Block<HttpRequestConfig, HttpRequestOutput> {
    validate(config: unknown): asserts config is HttpRequestConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "url");

        if ("method" in config && config.method !== undefined) {
            BlockHelpers.assertField(config, "method", "string");
        }

        if ("headers" in config && config.headers !== undefined) {
            BlockHelpers.assertField(config, "headers", "object");
        }
    }

    async execute(config: HttpRequestConfig): Promise<HttpRequestOutput> {
        const method = config.method || "GET";
        const headers = config.headers || {};
        const body = config.body ? JSON.stringify(config.body) : undefined;

        const response = await fetch(config.url, {
            method,
            headers,
            body,
        });

        const responseData = await response.text();

        let parsedData: unknown;
        try {
            parsedData = JSON.parse(responseData);
        } catch {
            parsedData = responseData;
        }

        return {
            status: response.status,
            statusText: response.statusText,
            headers: Object.fromEntries(response.headers.entries()),
            data: parsedData,
        };
    }

    protected getOutputPort(result: HttpRequestOutput): string {
        return BlockHelpers.getHttpPort(result.status);
    }
}

if (import.meta.main) {
    new HttpRequestBlock().run();
}
