// pkg/blocks/std/transform.test.ts
// Unit tests for Transform block

import { describe, test, expect } from "bun:test";
import {
    validateConfig,
    applyPick,
    applyRename,
    applyMap,
    applyJSONPath,
} from "./transform";

describe("Transform Block", () => {
    describe("validateConfig", () => {
        test("should pass for valid config with operations", () => {
            const config = {
                input: { a: 1 },
                operations: [{ type: 'pick', fields: ['a'] }]
            };
            expect(() => validateConfig(config)).not.toThrow();
        });

        test("should throw error when config is missing", () => {
            expect(() => validateConfig(null)).toThrow("Missing required config");
        });

        test("should throw error when operations is missing", () => {
            const config = { input: {} };
            expect(() => validateConfig(config)).toThrow("Config 'operations' must be an array");
        });

        test("should throw error when operations is empty", () => {
            const config = { input: {}, operations: [] };
            expect(() => validateConfig(config)).toThrow("At least one operation is required");
        });

        test("should throw error for operation without type", () => {
            const config = { input: {}, operations: [{ fields: ['a'] }] };
            expect(() => validateConfig(config)).toThrow("Each operation must have a 'type' field");
        });

        test("should throw error for unknown operation type", () => {
            const config = { input: {}, operations: [{ type: 'unknown' }] };
            expect(() => validateConfig(config)).toThrow("Unknown operation type");
        });
    });

    describe("applyPick", () => {
        test("should pick specified fields", () => {
            const data = { a: 1, b: 2, c: 3 };
            const result = applyPick(data, ['a', 'c']);
            expect(result).toEqual({ a: 1, c: 3 });
        });

        test("should ignore non-existent fields", () => {
            const data = { a: 1, b: 2 };
            const result = applyPick(data, ['a', 'nonexistent']);
            expect(result).toEqual({ a: 1 });
        });

        test("should return empty object for empty fields array", () => {
            const data = { a: 1, b: 2 };
            const result = applyPick(data, []);
            expect(result).toEqual({});
        });

        test("should throw error for non-object input", () => {
            expect(() => applyPick("not an object", ['a'])).toThrow("'pick' operation requires an object");
        });
    });

    describe("applyRename", () => {
        test("should rename fields", () => {
            const data = { oldName: 'value', other: 'data' };
            const result = applyRename(data, { oldName: 'newName' });
            expect(result).toEqual({ newName: 'value', other: 'data' });
        });

        test("should keep unmapped fields unchanged", () => {
            const data = { a: 1, b: 2, c: 3 };
            const result = applyRename(data, { a: 'x' });
            expect(result).toEqual({ x: 1, b: 2, c: 3 });
        });

        test("should handle empty mapping", () => {
            const data = { a: 1, b: 2 };
            const result = applyRename(data, {});
            expect(result).toEqual({ a: 1, b: 2 });
        });

        test("should throw error for non-object input", () => {
            expect(() => applyRename(null, {})).toThrow("'rename' operation requires an object");
        });
    });

    describe("applyMap", () => {
        test("should transform object", () => {
            const data = { value: 10 };
            const result = applyMap(data, "({ ...data, doubled: data.value * 2 })");
            expect(result).toEqual({ value: 10, doubled: 20 });
        });

        test("should transform array", () => {
            const data = [1, 2, 3];
            const result = applyMap(data, "data.map(x => x * 2)");
            expect(result).toEqual([2, 4, 6]);
        });

        test("should access nested properties", () => {
            const data = { user: { name: "John", age: 30 } };
            const result = applyMap(data, "data.user.name");
            expect(result).toBe("John");
        });

        test("should throw error for invalid expression", () => {
            const data = { value: 10 };
            expect(() => applyMap(data, "data.nonexistent.property")).toThrow("Map expression failed");
        });
    });

    describe("applyJSONPath", () => {
        test("should query simple path", () => {
            const data = { users: [{ name: "Alice" }, { name: "Bob" }] };
            const result = applyJSONPath(data, "$.users[0].name");
            expect(result).toBe("Alice");
        });

        test("should query array elements", () => {
            const data = { items: [1, 2, 3, 4, 5] };
            const result = applyJSONPath(data, "$.items[*]");
            expect(result).toEqual([1, 2, 3, 4, 5]);
        });

        test("should filter with expression", () => {
            const data = {
                products: [
                    { name: "A", price: 100 },
                    { name: "B", price: 50 },
                    { name: "C", price: 150 }
                ]
            };
            const result = applyJSONPath(data, "$.products[?@.price > 75]");
            expect(result).toEqual([
                { name: "A", price: 100 },
                { name: "C", price: 150 }
            ]);
        });

        test("should handle nested paths", () => {
            const data = {
                company: {
                    departments: [
                        { name: "IT", employees: 10 },
                        { name: "HR", employees: 5 }
                    ]
                }
            };
            const result = applyJSONPath(data, "$.company.departments[0].name");
            expect(result).toBe("IT");
        });

        test("should unwrap single-element arrays", () => {
            const data = { value: 42 };
            const result = applyJSONPath(data, "$.value");
            expect(result).toBe(42);
        });
    });

    describe("combined operations", () => {
        test("should apply multiple operations sequentially", () => {
            let data: any = {
                user: { firstName: "John", lastName: "Doe", age: 30, email: "john@example.com" }
            };

            data = applyJSONPath(data, "$.user");

            data = applyPick(data, ['firstName', 'lastName']);

            data = applyRename(data, { firstName: 'first', lastName: 'last' });

            expect(data).toEqual({ first: "John", last: "Doe" });
        });
    });
});
