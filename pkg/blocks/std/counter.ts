import { Block, BlockHelpers } from "#sdk";

interface CounterConfig {
    counterName: string;
    increment?: number;
    scope?: "global" | "workflow" | "execution";
    ttlSeconds?: number;
}

interface CounterOutput {
    currentValue: number;
    previousValue: number;
}

export class CounterBlock extends Block<CounterConfig, CounterOutput> {
    validate(config: unknown): asserts config is CounterConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "counterName");

        if ("increment" in config && typeof config.increment !== "number") {
            throw new Error("increment must be a number");
        }

        if ("scope" in config) {
            const validScopes = ["global", "workflow", "execution"];
            if (!validScopes.includes(config.scope as string)) {
                throw new Error(`scope must be one of: ${validScopes.join(", ")}`);
            }
        }
    }

    async execute(config: CounterConfig, input?: unknown): Promise<CounterOutput> {
        const increment = config.increment ?? 1;
        const scope = config.scope ?? "execution";
        const ttlSeconds = config.ttlSeconds;

        const inputData = input as { context?: { variables?: Record<string, unknown> } };
        const currentValue = (inputData?.context?.variables?.[config.counterName] as number) ?? 0;
        const newValue = currentValue + increment;

        this.variables.set(config.counterName, newValue, { scope, ttlSeconds });

        return {
            currentValue: newValue,
            previousValue: currentValue,
        };
    }
}

if (import.meta.main) {
    new CounterBlock().run();
}
