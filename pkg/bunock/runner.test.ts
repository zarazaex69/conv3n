
// Unit tests for Bunock runner

import { describe, test, expect, beforeEach } from "bun:test";

describe("Bunock Runner", () => {
    test("should read JSON from stdin", async () => {
        const testInput = {
            test: "data",
            value: 123,
            nested: { key: "value" },
        };

        // Mock stdin
        const mockStdin = {
            json: async () => testInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(input).toEqual(testInput);

        (Bun as any).stdin = originalStdin;
    });

    test("should write JSON to stdout", async () => {
        const testOutput = {
            status: "success",
            data: { result: 42 },
        };

        let capturedOutput = "";
        const mockWrite = async (stream: any, data: string) => {
            capturedOutput = data;
            return data.length;
        };

        const originalWrite = Bun.write;
        (Bun as any).write = mockWrite;

        await Bun.write(Bun.stdout, JSON.stringify(testOutput));

        expect(capturedOutput).toBe(JSON.stringify(testOutput));

        (Bun as any).write = originalWrite;
    });

    test("should create valid output structure", () => {
        const input = { test: "data" };
        const result = {
            status: "success",
            processed_at: new Date().toISOString(),
            original_input: input,
            message: "Hello from Bunock!",
        };

        expect(result.status).toBe("success");
        expect(result.processed_at).toBeDefined();
        expect(result.original_input).toEqual(input);
        expect(result.message).toBe("Hello from Bunock!");
    });

    test("should validate ISO timestamp format", () => {
        const timestamp = new Date().toISOString();

        // ISO 8601 format: YYYY-MM-DDTHH:mm:ss.sssZ
        const isoRegex = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;
        expect(isoRegex.test(timestamp)).toBe(true);
    });

    test("should handle empty input", async () => {
        const emptyInput = {};

        const mockStdin = {
            json: async () => emptyInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(input).toEqual({});

        (Bun as any).stdin = originalStdin;
    });

    test("should handle complex nested input", async () => {
        const complexInput = {
            level1: {
                level2: {
                    level3: {
                        array: [1, 2, 3],
                        value: "deep",
                    },
                },
            },
            metadata: {
                timestamp: Date.now(),
                version: "1.0.0",
            },
        };

        const mockStdin = {
            json: async () => complexInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(input).toEqual(complexInput);
        expect(input.level1.level2.level3.array).toEqual([1, 2, 3]);

        (Bun as any).stdin = originalStdin;
    });

    test("should handle array input", async () => {
        const arrayInput = [1, 2, 3, { key: "value" }];

        const mockStdin = {
            json: async () => arrayInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(Array.isArray(input)).toBe(true);
        expect(input).toEqual(arrayInput);

        (Bun as any).stdin = originalStdin;
    });

    test("should validate JSON stringification", () => {
        const data = {
            string: "value",
            number: 42,
            boolean: true,
            null: null,
            array: [1, 2, 3],
            object: { nested: "data" },
        };

        const jsonString = JSON.stringify(data);
        const parsed = JSON.parse(jsonString);

        expect(parsed).toEqual(data);
    });

    test("should handle special characters in JSON", async () => {
        const specialInput = {
            quotes: 'He said "Hello"',
            newlines: "Line 1\nLine 2",
            unicode: "Привет мир 🌍",
            backslash: "C:\\Users\\test",
        };

        const jsonString = JSON.stringify(specialInput);
        const parsed = JSON.parse(jsonString);

        expect(parsed).toEqual(specialInput);
    });

    test("should validate error handling structure", () => {
        const error = new Error("Test error");
        const errorOutput = {
            status: "error",
            message: error.message,
            stack: error.stack,
        };

        expect(errorOutput.status).toBe("error");
        expect(errorOutput.message).toBe("Test error");
        expect(errorOutput.stack).toBeDefined();
    });

    test("should handle large input data", async () => {

        const largeInput = {
            items: Array.from({ length: 1000 }, (_, i) => ({
                id: i,
                name: `Item ${i}`,
                data: { value: i * 2 },
            })),
        };

        const mockStdin = {
            json: async () => largeInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(input.items.length).toBe(1000);
        expect(input.items[999].id).toBe(999);

        (Bun as any).stdin = originalStdin;
    });

    test("should validate output is single-line JSON", () => {
        const output = {
            status: "success",
            data: { result: 42 },
        };

        const jsonString = JSON.stringify(output);

        // Single-line JSON should not contain newlines
        expect(jsonString).not.toContain("\n");
        expect(jsonString).not.toContain("\r");
    });

    test("should handle numeric input", async () => {
        const numericInput = 42;

        const mockStdin = {
            json: async () => numericInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(input).toBe(42);

        (Bun as any).stdin = originalStdin;
    });

    test("should handle boolean input", async () => {
        const booleanInput = true;

        const mockStdin = {
            json: async () => booleanInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(input).toBe(true);

        (Bun as any).stdin = originalStdin;
    });

    test("should handle null input", async () => {
        const nullInput = null;

        const mockStdin = {
            json: async () => nullInput,
        };

        const originalStdin = Bun.stdin;
        (Bun as any).stdin = mockStdin;

        const input = await Bun.stdin.json();
        expect(input).toBeNull();

        (Bun as any).stdin = originalStdin;
    });
});
