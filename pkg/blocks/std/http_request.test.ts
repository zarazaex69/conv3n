import { describe, test, expect, beforeEach, afterEach, mock } from "bun:test";
import { HttpRequestBlock } from "./http_request";
import { BlockHelpers } from "#sdk";

describe("HTTP Request Block", () => {
    let originalFetch: typeof global.fetch;

    beforeEach(() => {
        originalFetch = global.fetch;
    });

    afterEach(() => {
        global.fetch = originalFetch;
    });

    describe("validate", () => {
        test("should pass for valid config with url", () => {
            const block = new HttpRequestBlock();
            const config = { url: "https://example.com" };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should throw error when url is missing", () => {
            const block = new HttpRequestBlock();
            const config = { method: "GET" };
            expect(() => block.validate(config)).toThrow();
        });
    });

    describe("execute", () => {
        test("should make successful GET request", async () => {
            const mockResponse = {
                status: 200,
                statusText: "OK",
                headers: new Headers({ "Content-Type": "application/json" }),
                text: async () => JSON.stringify({ message: "success" }),
            };

            global.fetch = mock(async () => mockResponse as any) as any;

            const block = new HttpRequestBlock();
            const config = {
                url: "https://api.example.com/data",
                method: "GET",
            };

            const result = await block.execute(config);

            expect(result.status).toBe(200);
            expect(result.statusText).toBe("OK");
            expect(result.data).toEqual({ message: "success" });
        });

        test("should make successful POST request with body", async () => {
            const mockResponse = {
                status: 201,
                statusText: "Created",
                headers: new Headers({ "Content-Type": "application/json" }),
                text: async () => JSON.stringify({ id: 123 }),
            };

            global.fetch = mock(async (url: string, options?: any) => {
                expect(options.body).toBe(JSON.stringify({ name: "John Doe" }));
                return mockResponse as any;
            }) as any;

            const block = new HttpRequestBlock();
            const config = {
                url: "https://api.example.com/users",
                method: "POST",
                body: { name: "John Doe" },
            };

            const result = await block.execute(config);

            expect(result.status).toBe(201);
            expect(result.data).toEqual({ id: 123 });
        });
    });

    describe("getOutputPort", () => {
        test("should return success for 2xx status", () => {
            expect(BlockHelpers.getHttpPort(200)).toBe("success");
            expect(BlockHelpers.getHttpPort(201)).toBe("success");
        });

        test("should return client_error for 4xx status", () => {
            expect(BlockHelpers.getHttpPort(400)).toBe("client_error");
            expect(BlockHelpers.getHttpPort(404)).toBe("client_error");
        });

        test("should return server_error for 5xx status", () => {
            expect(BlockHelpers.getHttpPort(500)).toBe("server_error");
            expect(BlockHelpers.getHttpPort(503)).toBe("server_error");
        });
    });
});
