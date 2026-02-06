import { test, expect, describe } from "bun:test";
import { Block, BlockHelpers, isBlockInput, isBlockOutput } from "./sdk.ts";

// Mock block for testing
interface TestConfig {
    value: string;
    count: number;
}

interface TestOutput {
    result: string;
}

class TestBlock extends Block<TestConfig, TestOutput> {
    validate(config: unknown): asserts config is TestConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "value");
        BlockHelpers.assertField(config, "count", "number");
    }

    async execute(config: TestConfig, input?: unknown): Promise<TestOutput> {
        return {
            result: config.value.repeat(config.count),
        };
    }
}

// Mock block with custom port routing
class PortRoutingBlock extends Block<{ status: number }, { status: number }> {
    validate(config: unknown): asserts config is { status: number } {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "status", "number");
    }

    async execute(config: { status: number }): Promise<{ status: number }> {
        return { status: config.status };
    }

    protected getOutputPort(result: { status: number }): string {
        return BlockHelpers.getHttpPort(result.status);
    }
}

describe("Block", () => {
    test("execute returns correct result", async () => {
        const block = new TestBlock();
        const config: TestConfig = { value: "test", count: 3 };
        const result = await block.execute(config);

        expect(result.result).toBe("testtesttest");
    });

    test("validate throws on missing field", () => {
        const block: TestBlock = new TestBlock();

        expect(() => {
            block.validate({});
        }).toThrow("Missing required field: value");
    });

    test("validate throws on wrong type", () => {
        const block: TestBlock = new TestBlock();

        expect(() => {
            block.validate({ value: "test", count: "not a number" });
        }).toThrow("Field 'count' must be of type number");
    });

    test("validate throws on empty string", () => {
        const block: TestBlock = new TestBlock();

        expect(() => {
            block.validate({ value: "", count: 1 });
        }).toThrow("Field 'value' must not be empty");
    });

    test("getOutputPort returns custom port", async () => {
        const block = new PortRoutingBlock();
        const result = await block.execute({ status: 200 });
        const port = (block as any).getOutputPort(result);

        expect(port).toBe("success");
    });

    test("formatError handles Error objects", () => {
        const block = new TestBlock();
        const error = new Error("Test error");
        error.name = "TestError";

        const formatted = (block as any).formatError(error);

        expect(formatted.message).toBe("Test error");
        expect(formatted.type).toBe("TestError");
        expect(formatted.stack).toBeDefined();
    });

    test("formatError handles non-Error values", () => {
        const block = new TestBlock();
        const formatted = (block as any).formatError("string error");

        expect(formatted.message).toBe("string error");
        expect(formatted.type).toBe("UnknownError");
    });
});

describe("BlockHelpers", () => {
    describe("createSuccessResult", () => {
        test("creates result with default port", () => {
            const result = BlockHelpers.createSuccessResult({ value: 42 });

            expect(result.data).toEqual({ value: 42 });
            expect(result.port).toBe("default");
        });

        test("creates result with custom port", () => {
            const result = BlockHelpers.createSuccessResult({ value: 42 }, "custom");

            expect(result.port).toBe("custom");
        });
    });

    describe("createErrorResult", () => {
        test("creates error result with message", () => {
            const result = BlockHelpers.createErrorResult("Something failed");

            expect(result.data.error.message).toBe("Something failed");
            expect(result.data.error.type).toBe("Error");
            expect(result.port).toBe("error");
        });

        test("creates error result with custom type", () => {
            const result = BlockHelpers.createErrorResult("Validation failed", "ValidationError");

            expect(result.data.error.type).toBe("ValidationError");
        });

        test("creates error result with stack", () => {
            const result = BlockHelpers.createErrorResult("Error", "Error", "stack trace");

            expect(result.data.error.stack).toBe("stack trace");
        });
    });

    describe("assertObject", () => {
        test("passes for valid object", () => {
            expect(() => {
                BlockHelpers.assertObject({ key: "value" });
            }).not.toThrow();
        });

        test("throws for null", () => {
            expect(() => {
                BlockHelpers.assertObject(null);
            }).toThrow("config must be a non-null object");
        });

        test("throws for array", () => {
            expect(() => {
                BlockHelpers.assertObject([1, 2, 3]);
            }).toThrow("config must be a non-null object");
        });

        test("throws for primitive", () => {
            expect(() => {
                BlockHelpers.assertObject("string");
            }).toThrow("config must be a non-null object");
        });

        test("uses custom field name in error", () => {
            expect(() => {
                BlockHelpers.assertObject(null, "input");
            }).toThrow("input must be a non-null object");
        });
    });

    describe("assertField", () => {
        test("passes for correct string field", () => {
            expect(() => {
                BlockHelpers.assertField({ name: "test" }, "name", "string");
            }).not.toThrow();
        });

        test("passes for correct number field", () => {
            expect(() => {
                BlockHelpers.assertField({ count: 42 }, "count", "number");
            }).not.toThrow();
        });

        test("passes for correct boolean field", () => {
            expect(() => {
                BlockHelpers.assertField({ enabled: true }, "enabled", "boolean");
            }).not.toThrow();
        });

        test("passes for correct object field", () => {
            expect(() => {
                BlockHelpers.assertField({ config: { key: "value" } }, "config", "object");
            }).not.toThrow();
        });

        test("passes for correct array field", () => {
            expect(() => {
                BlockHelpers.assertField({ items: [1, 2, 3] }, "items", "array");
            }).not.toThrow();
        });

        test("throws for missing field", () => {
            expect(() => {
                BlockHelpers.assertField({}, "name", "string");
            }).toThrow("Missing required field: name");
        });

        test("throws for wrong type", () => {
            expect(() => {
                BlockHelpers.assertField({ count: "not a number" }, "count", "number");
            }).toThrow("Field 'count' must be of type number");
        });

        test("throws for array when expecting object", () => {
            expect(() => {
                BlockHelpers.assertField({ config: [1, 2] }, "config", "object");
            }).toThrow("Field 'config' must be an object");
        });

        test("throws for object when expecting array", () => {
            expect(() => {
                BlockHelpers.assertField({ items: { key: "value" } }, "items", "array");
            }).toThrow("Field 'items' must be an array");
        });
    });

    describe("assertNonEmptyString", () => {
        test("passes for non-empty string", () => {
            expect(() => {
                BlockHelpers.assertNonEmptyString({ url: "https://example.com" }, "url");
            }).not.toThrow();
        });

        test("throws for empty string", () => {
            expect(() => {
                BlockHelpers.assertNonEmptyString({ url: "" }, "url");
            }).toThrow("Field 'url' must not be empty");
        });

        test("throws for whitespace-only string", () => {
            expect(() => {
                BlockHelpers.assertNonEmptyString({ url: "   " }, "url");
            }).toThrow("Field 'url' must not be empty");
        });

        test("throws for non-string", () => {
            expect(() => {
                BlockHelpers.assertNonEmptyString({ url: 123 }, "url");
            }).toThrow("Field 'url' must be of type string");
        });
    });

    describe("parseJSON", () => {
        test("parses valid JSON", () => {
            const result = BlockHelpers.parseJSON('{"key":"value"}');
            expect(result).toEqual({ key: "value" });
        });

        test("returns raw string for invalid JSON", () => {
            const result = BlockHelpers.parseJSON("not json");
            expect(result).toBe("not json");
        });

        test("handles empty string", () => {
            const result = BlockHelpers.parseJSON("");
            expect(result).toBe("");
        });
    });

    describe("getHttpPort", () => {
        test("returns success for 2xx status", () => {
            expect(BlockHelpers.getHttpPort(200)).toBe("success");
            expect(BlockHelpers.getHttpPort(201)).toBe("success");
            expect(BlockHelpers.getHttpPort(299)).toBe("success");
        });

        test("returns client_error for 4xx status", () => {
            expect(BlockHelpers.getHttpPort(400)).toBe("client_error");
            expect(BlockHelpers.getHttpPort(404)).toBe("client_error");
            expect(BlockHelpers.getHttpPort(499)).toBe("client_error");
        });

        test("returns server_error for 5xx status", () => {
            expect(BlockHelpers.getHttpPort(500)).toBe("server_error");
            expect(BlockHelpers.getHttpPort(503)).toBe("server_error");
        });

        test("returns default for other status codes", () => {
            expect(BlockHelpers.getHttpPort(100)).toBe("default");
            expect(BlockHelpers.getHttpPort(300)).toBe("default");
        });
    });

    describe("getBooleanPort", () => {
        test("returns true for true value", () => {
            expect(BlockHelpers.getBooleanPort(true)).toBe("true");
        });

        test("returns false for false value", () => {
            expect(BlockHelpers.getBooleanPort(false)).toBe("false");
        });
    });
});

describe("Type Guards", () => {
    describe("isBlockInput", () => {
        test("returns true for valid BlockInput", () => {
            expect(isBlockInput({ config: { key: "value" } })).toBe(true);
        });

        test("returns true for BlockInput with input field", () => {
            expect(isBlockInput({ config: {}, input: "data" })).toBe(true);
        });

        test("returns false for null", () => {
            expect(isBlockInput(null)).toBe(false);
        });

        test("returns false for object without config", () => {
            expect(isBlockInput({ data: "value" })).toBe(false);
        });

        test("returns false for primitive", () => {
            expect(isBlockInput("string")).toBe(false);
        });
    });

    describe("isBlockOutput", () => {
        test("returns true for valid BlockOutput", () => {
            expect(isBlockOutput({ data: { key: "value" }, port: "default" })).toBe(true);
        });

        test("returns false for null", () => {
            expect(isBlockOutput(null)).toBe(false);
        });

        test("returns false for object without data", () => {
            expect(isBlockOutput({ port: "default" })).toBe(false);
        });

        test("returns false for object without port", () => {
            expect(isBlockOutput({ data: {} })).toBe(false);
        });

        test("returns false for object with non-string port", () => {
            expect(isBlockOutput({ data: {}, port: 123 })).toBe(false);
        });
    });
});

// Integration-style tests for Block.run to cover stdin/stdout and error handling
describe("Block integration with stdin/stdout", () => {
    test("run writes success output with default port", async () => {
        class InlineBlock extends Block<{ message: string }, { echo: string }> {
            validate(config: unknown): asserts config is { message: string } {
                BlockHelpers.assertObject(config);
                BlockHelpers.assertField(config, "message", "string");
            }

            async execute(config: { message: string }): Promise<{ echo: string }> {
                return { echo: config.message };
            }
        }

        const originalStdin = Bun.stdin;
        const originalWrite = Bun.write;

        try {
            // Mock stdin
            (Bun as any).stdin = {
                json: async () => ({
                    config: { message: "hello" },
                }),
            };

            let written = "";
            (Bun as any).write = async (_stream: any, data: string) => {
                written = data;
                return data.length;
            };

            const block = new InlineBlock();
            await block.run();

            const parsed = JSON.parse(written);
            expect(parsed.port).toBe("default");
            expect(parsed.data.echo).toBe("hello");
        } finally {
            (Bun as any).stdin = originalStdin;
            (Bun as any).write = originalWrite;
        }
    });

    test("run uses writeError when validate throws", async () => {
        class FailingBlock extends Block<{ required: string }, { ok: boolean }> {
            validate(_config: unknown): asserts _config is { required: string } {
                throw new Error("validation failed");
            }

            async execute(): Promise<{ ok: boolean }> {
                return { ok: true };
            }
        }

        const originalStdin = Bun.stdin;
        const originalWrite = Bun.write;
        const originalExit = process.exit;

        try {
            (Bun as any).stdin = {
                json: async () => ({
                    config: {},
                }),
            };

            let written = "";
            (Bun as any).write = async (_stream: any, data: string) => {
                written = data;
                return data.length;
            };

            // Prevent the real process from exiting during the test
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            (process as any).exit = () => {
                // no-op in tests
            };

            const block = new FailingBlock();
            await block.run();

            const parsed = JSON.parse(written);
            expect(parsed.port).toBe("error");
            expect(parsed.data.error.message).toBe("validation failed");
            expect(parsed.data.error.type).toBe("Error");
        } finally {
            (Bun as any).stdin = originalStdin;
            (Bun as any).write = originalWrite;
            (process as any).exit = originalExit;
        }
    });
});
