import { Block } from "../../bunock/sdk/sdk.ts";

interface ExampleConfig {
    message: string;
    multiplier?: number;
}

interface ExampleOutput {
    result: string;
    count: number;
}

export class ExampleBlock extends Block<ExampleConfig, ExampleOutput> {
    validate(config: unknown): asserts config is ExampleConfig {
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

    async execute(config: ExampleConfig, input?: unknown): Promise<ExampleOutput> {
        const multiplier = config.multiplier || 1;
        const result = config.message.repeat(multiplier);

        return {
            result,
            count: result.length,
        };
    }
}

if (import.meta.main) {
    new ExampleBlock().run();
}
