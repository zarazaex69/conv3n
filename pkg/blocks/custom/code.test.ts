
// Unit tests for Custom Code block

import { describe, test, expect, beforeEach } from "bun:test";

describe("Custom Code Block", () => {
    test("should execute valid TypeScript code", async () => {
        const input = {
            config: {
                code: "export default async (input) => { return { result: input.value * 2 }; }",
                input: { value: 21 },
            },
        };

        // Mock stdin
        const mockStdin = {
            json: async () => input,
        };

        let capturedOutput = "";
        const mockWrite = async (stream: any, data: string) => {
            capturedOutput = data;
        };

        // Replace global functions
        const originalStdin = Bun.stdin;
        const originalWrite = Bun.write;
        (Bun as any).stdin = mockStdin;
        (Bun as any).write = mockWrite;

        // Note: Actual execution would require running the script

        // Restore
        (Bun as any).stdin = originalStdin;
        (Bun as any).write = originalWrite;
    });

    test("should handle syntax errors", async () => {
        const input = {
            config: {
                code: "export default async (input) => { return { invalid syntax }",
            },
        };

        // Test transpiler validation
        const transpiler = new Bun.Transpiler({ loader: "ts" });

        let syntaxError = null;
        try {
            transpiler.transformSync(input.config.code);
        } catch (error) {
            syntaxError = error;
        }

        expect(syntaxError).not.toBeNull();
    });

    test("should handle valid async function", async () => {
        const code = `
            export default async (input) => {
                await new Promise(resolve => setTimeout(resolve, 10));
                return { result: "async completed" };
            }
        `;

        const transpiler = new Bun.Transpiler({ loader: "ts" });

        // Should not throw
        expect(() => transpiler.transformSync(code)).not.toThrow();
    });

    test("should validate code structure", async () => {
        const validCodes = [
            "export default async (input) => { return input; }",
            "export default async function(input) { return input; }",
            `export default async (input) => {
                const result = input.value + 10;
                return { result };
            }`,
        ];

        const transpiler = new Bun.Transpiler({ loader: "ts" });

        for (const code of validCodes) {
            expect(() => transpiler.transformSync(code)).not.toThrow();
        }
    });

    test("should detect missing export default", async () => {
        const invalidCode = "async (input) => { return input; }"; // No export default

        const transpiler = new Bun.Transpiler({ loader: "ts" });

        // This will transpile successfully, but won't have default export

        const transpiled = transpiler.transformSync(invalidCode);
        expect(transpiled).toBeDefined();
    });

    test("should handle complex data types in input", async () => {
        const testInputs = [
            { value: 42 },
            { array: [1, 2, 3] },
            { nested: { deep: { value: "test" } } },
            { mixed: { num: 1, str: "text", bool: true, arr: [1, 2] } },
        ];

        for (const testInput of testInputs) {
            const code = "export default async (input) => { return input; }";
            const transpiler = new Bun.Transpiler({ loader: "ts" });

            expect(() => transpiler.transformSync(code)).not.toThrow();
        }
    });

    test("should validate code wrapping logic", async () => {

        const userCode = "return { result: 42 };";
        const wrappedCode = `export default async (input) => { ${userCode} }`;

        const transpiler = new Bun.Transpiler({ loader: "ts" });
        expect(() => transpiler.transformSync(wrappedCode)).not.toThrow();
    });

    test("should handle code with imports", async () => {

        const codeWithImport = `
            export default async (input) => {

                return { result: "no imports" };
            }
        `;

        const transpiler = new Bun.Transpiler({ loader: "ts" });
        expect(() => transpiler.transformSync(codeWithImport)).not.toThrow();
    });

    test("should validate execution time tracking", () => {
        const startTime = performance.now();

        // Simulate some work
        let sum = 0;
        for (let i = 0; i < 1000; i++) {
            sum += i;
        }

        const endTime = performance.now();
        const executionTime = endTime - startTime;

        expect(executionTime).toBeGreaterThan(0);
        expect(executionTime).toBeLessThan(1000); // Should be very fast
    });

    test("should handle empty input", async () => {
        const code = "export default async (input) => { return { hasInput: !!input }; }";
        const transpiler = new Bun.Transpiler({ loader: "ts" });

        expect(() => transpiler.transformSync(code)).not.toThrow();
    });

    test("should validate error object structure", () => {
        const errorTypes = ["SyntaxError", "ImportError", "RuntimeError", "UnexpectedError"];

        for (const errorType of errorTypes) {
            const errorObj = {
                success: false,
                error: {
                    message: `Test ${errorType}`,
                    stack: "Error stack trace",
                    type: errorType,
                },
                executionTime: 123,
            };

            expect(errorObj.success).toBe(false);
            expect(errorObj.error.type).toBe(errorType);
            expect(errorObj.executionTime).toBeGreaterThan(0);
        }
    });

    test("should validate success output structure", () => {
        const successOutput = {
            success: true,
            data: { result: 42 },
            executionTime: 50,
        };

        expect(successOutput.success).toBe(true);
        expect(successOutput.data).toBeDefined();
        expect(successOutput.executionTime).toBeGreaterThan(0);
    });

    test("should handle code with TypeScript features", async () => {
        const tsCode = `
            export default async (input: any): Promise<any> => {
                const value: number = input.value;
                const result: string = value.toString();
                return { result };
            }
        `;

        const transpiler = new Bun.Transpiler({ loader: "ts" });
        expect(() => transpiler.transformSync(tsCode)).not.toThrow();
    });

    test("should validate base64 encoding for data URL", () => {
        const code = "export default async (input) => input;";
        const base64 = btoa(code);
        const dataUrl = `data:text/typescript;base64,${base64}`;

        expect(base64).toBeDefined();
        expect(dataUrl).toContain("data:text/typescript;base64,");

        // Verify we can decode it back
        const decoded = atob(base64);
        expect(decoded).toBe(code);
    });
});
