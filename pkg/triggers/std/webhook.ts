import { Trigger, type TriggerContext, type TriggerMessage, BlockHelpers } from "#sdk";

interface WebhookTriggerConfig extends Record<string, unknown> {}

class WebhookTrigger extends Trigger<WebhookTriggerConfig> {
    validate(config: unknown): asserts config is WebhookTriggerConfig {
        BlockHelpers.assertObject(config);
    }

    async start(ctx: TriggerContext<WebhookTriggerConfig>): Promise<void> {
        console.log("Webhook trigger started, awaiting invocations...");

        this.registerMessageHandler("invoke", async (message: TriggerMessage) => {
            console.log("Webhook invocation received");
            await ctx.fire(message.payload);
        });
    }

    async stop(ctx: TriggerContext<WebhookTriggerConfig>): Promise<void> {
        console.log("Webhook trigger stopped");
    }
}

if (import.meta.main) {
    new WebhookTrigger().run();
}
