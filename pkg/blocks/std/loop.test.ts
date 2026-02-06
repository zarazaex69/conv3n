import { describe, test, expect } from "bun:test";
import { LoopBlock } from "./loop";

describe("Loop Block", () => {
    describe("validate", () => {
        test("should pass for valid config with items array", () => {
            const block = new LoopBlock();
            const config = { items: [1, 2, 3] };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should throw error when items is not an array", () => {
            const block = new LoopBlock();
            const config = { items: "not an array" };
            expect(() => block.validate(config)).toThrow();
        });

        test("should pass with optional map expression", () => {
            const block = new LoopBlock();
            const config = { items: [1, 2, 3], mapExpression: "item * 2" };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should pass with optional filter expression", () => {
            const block = new LoopBlock();
            const config = { items: [1, 2, 3], filterExpression: "item > 1" };
            expect(() => block.validate(config)).not.toThrow();
        });
    });

    describe("execute", () => {
        test("should process items without transformations", async () => {
            const block = new LoopBlock();
            const result = await block.execute({ items: [1, 2, 3] });

            expect(result.results).toEqual([1, 2, 3]);
            expect(result.count).toBe(3);
            expect(result.originalCount).toBe(3);
        });

        test("should apply map expression", async () => {
            const block = new LoopBlock();
            const result = await block.execute({
                items: [1, 2, 3],
                mapExpression: "item * 2",
            });

            expect(result.results).toEqual([2, 4, 6]);
            expect(result.count).toBe(3);
        });

        test("should apply filter expression", async () => {
            const block = new LoopBlock();
            const result = await block.execute({
                items: [1, 2, 3, 4, 5],
                filterExpression: "item > 2",
            });

            expect(result.results).toEqual([3, 4, 5]);
            expect(result.count).toBe(3);
            expect(result.originalCount).toBe(5);
        });

        test("should apply both filter and map", async () => {
            const block = new LoopBlock();
            const result = await block.execute({
                items: [1, 2, 3, 4, 5],
                filterExpression: "item > 2",
                mapExpression: "item * 2",
            });

            expect(result.results).toEqual([6, 8, 10]);
            expect(result.count).toBe(3);
        });

        test("should handle empty array", async () => {
            const block = new LoopBlock();
            const result = await block.execute({ items: [] });

            expect(result.results).toEqual([]);
            expect(result.count).toBe(0);
            expect(result.originalCount).toBe(0);
        });
    });
});
