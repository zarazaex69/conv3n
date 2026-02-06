import { describe, test, expect, beforeAll, afterAll } from "bun:test";
import { WebhookBlock } from "./webhook";
import type { Server } from "bun";

let mockServer: Server<any>;
let mockServerUrl: string;

describe("Webhook Block", () => {
    beforeAll(() => {
        mockServer = Bun.serve({
            port: 0,
            fetch(req) {
                const url = new URL(req.url);

                if (url.pathname === "/success") {
                    return new Response(JSON.stringify({ message: "Success" }), {
                        status: 200,
                        headers: { "Content-Type": "application/json" },
                    });
                }

                if (url.pathname === "/created") {
                    return new Response(JSON.stringify({ id: 123 }), {
                        status: 201,
                        headers: { "Content-Type": "application/json" },
                    });
                }

                return new Response("Not Found", { status: 404 });
            },
        });

        mockServerUrl = `http://localhost:${mockServer.port}`;
    });

    afterAll(() => {
        mockServer.stop();
    });

    describe("validate", () => {
        test("should pass for valid config", () => {
            const block = new WebhookBlock();
            const config = {
                url: "https://example.com/webhook",
                method: "POST" as const,
            };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should throw error for invalid URL", () => {
            const block = new WebhookBlock();
            const config = {
                url: "not-a-url",
                method: "POST" as const,
            };
            expect(() => block.validate(config)).toThrow("Invalid URL format");
        });

        test("should throw error for invalid method", () => {
            const block = new WebhookBlock();
            const config = {
                url: "https://example.com",
                method: "GET",
            };
            expect(() => block.validate(config)).toThrow("Invalid HTTP method");
        });
    });

    describe("execute", () => {
        test("should make successful POST request", async () => {
            const block = new WebhookBlock();
            const result = await block.execute({
                url: `${mockServerUrl}/success`,
                method: "POST",
            });

            expect(result.status).toBe(200);
            expect(result.data).toEqual({ message: "Success" });
            expect(result.duration).toBeGreaterThan(0);
        });

        test("should make POST request with body", async () => {
            const block = new WebhookBlock();
            const result = await block.execute({
                url: `${mockServerUrl}/created`,
                method: "POST",
                body: { name: "Test" },
            });

            expect(result.status).toBe(201);
            expect(result.data).toEqual({ id: 123 });
        });

        test("should handle custom headers", async () => {
            const block = new WebhookBlock();
            const result = await block.execute({
                url: `${mockServerUrl}/success`,
                method: "POST",
                headers: { "X-Custom": "value" },
            });

            expect(result.status).toBe(200);
        });

        test("should handle timeout", async () => {
            const block = new WebhookBlock();

            const slowServer = Bun.serve({
                port: 0,
                async fetch() {
                    await Bun.sleep(5000);
                    return new Response("OK");
                },
            });

            try {
                await expect(
                    block.execute({
                        url: `http://localhost:${slowServer.port}/slow`,
                        method: "POST",
                        timeout: 100,
                    })
                ).rejects.toThrow("timeout");
            } finally {
                slowServer.stop();
            }
        });
    });
});
