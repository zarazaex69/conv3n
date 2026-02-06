
// Unit tests for Condition block

import { describe, test, expect } from "bun:test";
import {
    validateExpression,
    evaluateExpression,
    ConditionBlock,
    type ConditionConfig,
} from "./condition";

describe("Condition Block", () => {
    describe("validateExpression", () => {
        test("should pass for valid simple expression", () => {
            expect(() => validateExpression("input.value > 10")).not.toThrow();
        });

        test("should pass for valid complex expression", () => {
            expect(() => validateExpression("input.a > 10 && input.b < 20")).not.toThrow();
        });

        test("should pass for boolean literals", () => {
            expect(() => validateExpression("true")).not.toThrow();
            expect(() => validateExpression("false")).not.toThrow();
        });

        test("should throw error for invalid syntax", () => {
            expect(() => validateExpression("input.value >")).toThrow("Invalid expression syntax");
        });

        test("should throw error for unclosed parenthesis", () => {
            expect(() => validateExpression("(input.value > 10")).toThrow("Invalid expression syntax");
        });

        test("should pass for function calls", () => {
            expect(() => validateExpression("input.items.length > 0")).not.toThrow();
        });
    });

    describe("evaluateExpression", () => {
        test("should evaluate simple comparison (true)", () => {
            const context = { value: 15 };
            const result = evaluateExpression("input.value > 10", context);
            expect(result).toBe(true);
        });

        test("should evaluate simple comparison (false)", () => {
            const context = { value: 5 };
            const result = evaluateExpression("input.value > 10", context);
            expect(result).toBe(false);
        });

        test("should evaluate equality check", () => {
            const context = { status: "completed" };
            expect(evaluateExpression("input.status === 'completed'", context)).toBe(true);
            expect(evaluateExpression("input.status === 'pending'", context)).toBe(false);
        });

        test("should evaluate boolean AND", () => {
            const context = { a: 15, b: 5 };
            expect(evaluateExpression("input.a > 10 && input.b < 10", context)).toBe(true);
            expect(evaluateExpression("input.a > 10 && input.b > 10", context)).toBe(false);
        });

        test("should evaluate boolean OR", () => {
            const context = { a: 15, b: 5 };
            expect(evaluateExpression("input.a > 10 || input.b > 10", context)).toBe(true);
            expect(evaluateExpression("input.a < 10 || input.b > 10", context)).toBe(false);
        });

        test("should evaluate NOT operator", () => {
            const context = { active: false };
            expect(evaluateExpression("!input.active", context)).toBe(true);
        });

        test("should handle nested object access", () => {
            const context = { user: { age: 25, name: "John" } };
            expect(evaluateExpression("input.user.age >= 18", context)).toBe(true);
            expect(evaluateExpression("input.user.name === 'John'", context)).toBe(true);
        });

        test("should handle array length checks", () => {
            const context = { items: [1, 2, 3] };
            expect(evaluateExpression("input.items.length > 0", context)).toBe(true);
            expect(evaluateExpression("input.items.length === 3", context)).toBe(true);
        });

        test("should coerce to boolean", () => {
            const context = { value: 1 };
            expect(evaluateExpression("input.value", context)).toBe(true);
        });

        test("should handle falsy values", () => {
            expect(evaluateExpression("input.value", { value: 0 })).toBe(false);
            expect(evaluateExpression("input.value", { value: "" })).toBe(false);
            expect(evaluateExpression("input.value", { value: null })).toBe(false);
            expect(evaluateExpression("input.value", { value: undefined })).toBe(false);
        });

        test("should handle empty context", () => {
            const context = {};
            expect(evaluateExpression("true", context)).toBe(true);
            expect(evaluateExpression("false", context)).toBe(false);
        });

        test("should throw error for runtime errors", () => {
            const context = { value: 10 };
            expect(() => evaluateExpression("input.nonexistent.property", context)).toThrow();
        });

        test("should handle ternary operator", () => {
            const context = { value: 15 };
            expect(evaluateExpression("input.value > 10 ? true : false", context)).toBe(true);
        });

        test("should handle typeof checks", () => {
            const context = { value: "hello" };
            expect(evaluateExpression("typeof input.value === 'string'", context)).toBe(true);
        });

        test("should handle null/undefined checks", () => {
            const context = { value: null };
            expect(evaluateExpression("input.value === null", context)).toBe(true);
            expect(evaluateExpression("input.value !== undefined", context)).toBe(true);
        });

        test("should handle numeric comparisons", () => {
            const context = { price: 99.99 };
            expect(evaluateExpression("input.price < 100", context)).toBe(true);
            expect(evaluateExpression("input.price >= 99", context)).toBe(true);
        });

        test("should handle string methods", () => {
            const context = { text: "Hello World" };
            expect(evaluateExpression("input.text.includes('World')", context)).toBe(true);
            expect(evaluateExpression("input.text.startsWith('Hello')", context)).toBe(true);
        });

        test("should handle array methods", () => {
            const context = { tags: ["javascript", "typescript", "bun"] };
            expect(evaluateExpression("input.tags.includes('bun')", context)).toBe(true);
            expect(evaluateExpression("input.tags.some(tag => tag.startsWith('type'))", context)).toBe(true);
        });
    });

    describe("ConditionBlock Class", () => {
        test("should validate correct config", () => {
            const block = new ConditionBlock();

            expect(() => block.validate({ expression: "true" })).not.toThrow();
        });

        test("should throw on missing expression", () => {
            const block = new ConditionBlock();

            expect(() => block.validate({})).toThrow();
        });

        test("should throw on invalid expression syntax in validate", () => {
            const block = new ConditionBlock();

            expect(() => block.validate({ expression: "input. >" })).toThrow("Invalid expression syntax");
        });

        test("should execute and return result", async () => {
            const block = new ConditionBlock();
            const config = { expression: "input.val > 5" };
            const input = { val: 10 };

            const result = await block.execute(config, input);
            expect(result.result).toBe(true);
            expect(result.expression).toBe(config.expression);
        });

        test("should route to 'true' port", () => {
            const block = new ConditionBlock();

            const port = block.getOutputPort({ result: true, expression: "" });
            expect(port).toBe("true");
        });

        test("should route to 'false' port", () => {
            const block = new ConditionBlock();

            const port = block.getOutputPort({ result: false, expression: "" });
            expect(port).toBe("false");
        });
    });
});
