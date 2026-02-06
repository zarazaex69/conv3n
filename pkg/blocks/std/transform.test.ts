import { describe, test, expect } from "bun:test";
import { TransformBlock } from "./transform";

describe("Transform Block", () => {
    describe("validate", () => {
        test("should pass for valid config with operations", () => {
            const block = new TransformBlock();
            const config = {
                input: { a: 1 },
                operations: [{ type: "pick", fields: ["a"] }],
            };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should throw error when config is missing", () => {
            const block = new TransformBlock();
            expect(() => block.validate(null)).toThrow();
        });

        test("should throw error when operations is missing", () => {
            const block = new TransformBlock();
            const config = { input: {} };
            expect(() => block.validate(config)).toThrow();
        });

        test("should throw error when operations is empty", () => {
            const block = new TransformBlock();
            const config = { input: {}, operations: [] };
            expect(() => block.validate(config)).toThrow("At least one operation is required");
        });

        test("should throw error for unknown operation type", () => {
            const block = new TransformBlock();
            const config = { input: {}, operations: [{ type: "unknown" }] };
            expect(() => block.validate(config)).toThrow("Unknown operation type");
        });
    });

    describe("execute", () => {
        test("should apply pick operation", async () => {
            const block = new TransformBlock();
            const config = {
                input: { a: 1, b: 2, c: 3 },
                operations: [{ type: "pick", fields: ["a", "c"] }],
            };
            const result = await block.execute(config);
            expect(result.data).toEqual({ a: 1, c: 3 });
            expect(result.operationsApplied).toBe(1);
        });

        test("should apply rename operation", async () => {
            const block = new TransformBlock();
            const config = {
                input: { oldName: "value", other: "data" },
                operations: [{ type: "rename", mapping: { oldName: "newName" } }],
            };
            const result = await block.execute(config);
            expect(result.data).toEqual({ newName: "value", other: "data" });
        });

        test("should apply map operation", async () => {
            const block = new TransformBlock();
            const config = {
                input: { value: 10 },
                operations: [{ type: "map", expression: "({ ...data, doubled: data.value * 2 })" }],
            };
            const result = await block.execute(config);
            expect(result.data).toEqual({ value: 10, doubled: 20 });
        });

        test("should apply jsonpath operation", async () => {
            const block = new TransformBlock();
            const config = {
                input: { users: [{ name: "Alice" }, { name: "Bob" }] },
                operations: [{ type: "jsonpath", query: "$.users[0].name" }],
            };
            const result = await block.execute(config);
            expect(result.data).toBe("Alice");
        });

        test("should apply multiple operations sequentially", async () => {
            const block = new TransformBlock();
            const config = {
                input: {
                    user: { firstName: "John", lastName: "Doe", age: 30, email: "john@example.com" },
                },
                operations: [
                    { type: "jsonpath", query: "$.user" },
                    { type: "pick", fields: ["firstName", "lastName"] },
                    { type: "rename", mapping: { firstName: "first", lastName: "last" } },
                ],
            };
            const result = await block.execute(config);
            expect(result.data).toEqual({ first: "John", last: "Doe" });
            expect(result.operationsApplied).toBe(3);
        });
    });
});
