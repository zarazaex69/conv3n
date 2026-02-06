import { test, expect, mock } from "bun:test";
import { Trigger, type TriggerContext } from "./trigger.ts";
import { BlockHelpers } from "./sdk.ts";

interface TestConfig extends Record<string, unknown> {
    value: string;
}

class TestTrigger extends Trigger<TestConfig> {
    public startCalled = false;
    public stopCalled = false;
    public firedPayloads: unknown[] = [];

    validate(config: unknown): asserts config is TestConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "value");
    }

    async start(ctx: TriggerContext<TestConfig>): Promise<void> {
        this.startCalled = true;
        this.firedPayloads.push(ctx.config.value);
    }

    async stop(ctx: TriggerContext<TestConfig>): Promise<void> {
        this.stopCalled = true;
    }
}

test("Trigger validates config", () => {
    const trigger: TestTrigger = new TestTrigger();

    expect(() => {
        trigger.validate({});
    }).toThrow("Missing required field: value");

    expect(() => {
        trigger.validate({ value: "" });
    }).toThrow("must not be empty");

    expect(() => {
        trigger.validate({ value: "test" });
    }).not.toThrow();
});

test("Trigger context provides config", async () => {
    const trigger: TestTrigger = new TestTrigger();
    const config = { value: "test-value" };

    trigger.validate(config);

    const ctx: TriggerContext<TestConfig> = {
        config,
        fire: async (payload) => payload
    };

    await trigger.start(ctx);

    expect(trigger.startCalled).toBe(true);
    expect(trigger.firedPayloads).toContain("test-value");
});

test("Trigger stop is called", async () => {
    const trigger = new TestTrigger();
    const config = { value: "test" };

    const ctx: TriggerContext<TestConfig> = {
        config,
        fire: async (payload) => payload
    };

    await trigger.stop(ctx);

    expect(trigger.stopCalled).toBe(true);
});

test("Trigger fire function works", async () => {
    const trigger = new TestTrigger();
    const firedPayloads: unknown[] = [];

    const ctx: TriggerContext<TestConfig> = {
        config: { value: "test" },
        fire: async (payload) => {
            firedPayloads.push(payload);
            return { success: true };
        }
    };

    const result = await ctx.fire({ data: "test-payload" });

    expect(firedPayloads).toHaveLength(1);
    expect(firedPayloads[0]).toEqual({ data: "test-payload" });
    expect(result).toEqual({ success: true });
});
