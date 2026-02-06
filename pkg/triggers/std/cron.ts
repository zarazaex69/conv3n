import { Trigger, type TriggerContext, BlockHelpers } from "#sdk";
import { Cron } from "croner";

interface CronTriggerConfig extends Record<string, unknown> {
    schedule: string;
}

class CronTrigger extends Trigger<CronTriggerConfig> {
    private cronJob: InstanceType<typeof Cron> | null = null;

    validate(config: unknown): asserts config is CronTriggerConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "schedule");
    }

    async start(ctx: TriggerContext<CronTriggerConfig>): Promise<void> {
        const { schedule } = ctx.config;

        try {
            this.cronJob = new Cron(schedule, async () => {
                await ctx.fire({
                    schedule,
                    timestamp: Date.now(),
                    triggeredAt: new Date().toISOString()
                });
            });

            console.log(`Cron trigger started: ${schedule}`);
        } catch (error) {
            throw new Error(`Failed to start cron: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    async stop(ctx: TriggerContext<CronTriggerConfig>): Promise<void> {
        if (this.cronJob) {
            this.cronJob.stop();
            this.cronJob = null;
            console.log(`Cron trigger stopped: ${ctx.config.schedule}`);
        }
    }
}

if (import.meta.main) {
    new CronTrigger().run();
}