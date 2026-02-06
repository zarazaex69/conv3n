import { describe, test, expect } from "bun:test";
import { DelayBlock } from "./delay";

describe("Delay Block", () => {
    describe("validate", () => {
        test("should pass for valid config with duration in ms", () => {
            const block = new DelayBlock();
            const config = { duration: 100 };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should pass for valid config with duration in seconds", () => {
            const block = new DelayBlock();
            const config = { duration: 2, unit: "s" as const };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should throw error when duration is negative", () => {
            const block = new DelayBlock();
            const config = { duration: -100 };
            expect(() => block.validate(config)).toThrow("duration must be non-negative");
        });

        test("should throw error for invalid unit", () => {
            const block = new DelayBlock();
            const config = { duration: 100, unit: "minutes" };
            expect(() => block.validate(config)).toThrow("unit must be 'ms' or 's'");
        });
    });

    describe("execute", () => {
        test("should execute delay in milliseconds", async () => {
            const block = new DelayBlock();
            const config = { duration: 50 };
            const startTime = Date.now();
            const result = await block.execute(config);
            const elapsed = Date.now() - startTime;

            expect(result.delayed).toBeGreaterThanOrEqual(45);
            expect(result.delayed).toBeLessThan(100);
            expect(result.unit).toBe("ms");
            expect(result.timestamp).toMatch(/^\d{4}-\d{2}-\d{2}T/);
            expect(elapsed).toBeGreaterThanOrEqual(45);
        });

        test("should execute delay in seconds", async () => {
            const block = new DelayBlock();
            const config = { duration: 0.1, unit: "s" as const };
            const startTime = Date.now();
            const result = await block.execute(config);
            const elapsed = Date.now() - startTime;

            expect(result.delayed).toBeGreaterThanOrEqual(95);
            expect(result.delayed).toBeLessThan(150);
            expect(result.unit).toBe("s");
            expect(elapsed).toBeGreaterThanOrEqual(95);
        });

        test("should handle zero delay", async () => {
            const block = new DelayBlock();
            const config = { duration: 0 };
            const result = await block.execute(config);

            expect(result.delayed).toBeGreaterThanOrEqual(0);
            expect(result.delayed).toBeLessThan(10);
            expect(result.unit).toBe("ms");
        });
    });
});
