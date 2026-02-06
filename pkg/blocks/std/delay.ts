import { Block, BlockHelpers } from "#sdk";

interface DelayConfig {
    duration: number;
    unit?: "ms" | "s";
}

interface DelayOutput {
    delayed: number;
    timestamp: string;
    unit: string;
}

export class DelayBlock extends Block<DelayConfig, DelayOutput> {
    validate(config: unknown): asserts config is DelayConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "duration", "number");

        const duration = config.duration as number;
        if (duration < 0) {
            throw new Error("duration must be non-negative");
        }

        if ("unit" in config && config.unit !== undefined) {
            const unit = config.unit;
            if (unit !== "ms" && unit !== "s") {
                throw new Error("unit must be 'ms' or 's'");
            }
        }
    }

    async execute(config: DelayConfig): Promise<DelayOutput> {
        const unit = config.unit ?? "ms";
        const durationMs = unit === "s" ? config.duration * 1000 : config.duration;

        const startTime = Date.now();
        await Bun.sleep(durationMs);
        const actualDelay = Date.now() - startTime;

        return {
            delayed: actualDelay,
            timestamp: new Date().toISOString(),
            unit,
        };
    }
}

if (import.meta.main) {
    new DelayBlock().run();
}
