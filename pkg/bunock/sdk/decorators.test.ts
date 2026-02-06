import { describe, test, expect } from "bun:test";
import {
    withTimeout,
    withRetry,
    executeWithTimeout,
    executeWithRetry,
    executeWithTimeoutAndRetry,
} from "./decorators.ts";

describe("Timeout utilities", () => {
    test("withTimeout resolves if promise completes in time", async () => {
        const promise = new Promise(resolve => setTimeout(() => resolve("success"), 10));
        const result = await withTimeout(promise, { ms: 100 });
        expect(result).toBe("success");
    });

    test("withTimeout rejects if promise takes too long", async () => {
        const promise = new Promise(resolve => setTimeout(() => resolve("success"), 200));

        await expect(
            withTimeout(promise, { ms: 50 })
        ).rejects.toThrow("Operation timed out after 50ms");
    });

    test("withTimeout uses custom error message", async () => {
        const promise = new Promise(resolve => setTimeout(() => resolve("success"), 200));

        await expect(
            withTimeout(promise, { ms: 50, errorMessage: "Custom timeout error" })
        ).rejects.toThrow("Custom timeout error");
    });

    test("executeWithTimeout helper works", async () => {
        const result = await executeWithTimeout(
            async () => {
                await new Promise(resolve => setTimeout(resolve, 10));
                return "done";
            },
            100
        );
        expect(result).toBe("done");
    });
});

describe("Retry utilities", () => {
    test("withRetry succeeds on first attempt", async () => {
        let attempts = 0;
        const result = await withRetry(
            async () => {
                attempts++;
                return "success";
            },
            { attempts: 3 }
        );

        expect(result).toBe("success");
        expect(attempts).toBe(1);
    });

    test("withRetry retries on failure", async () => {
        let attempts = 0;
        const result = await withRetry(
            async () => {
                attempts++;
                if (attempts < 3) {
                    throw new Error("Temporary failure");
                }
                return "success";
            },
            { attempts: 3 }
        );

        expect(result).toBe("success");
        expect(attempts).toBe(3);
    });

    test("withRetry throws after all attempts fail", async () => {
        let attempts = 0;

        await expect(
            withRetry(
                async () => {
                    attempts++;
                    throw new Error("Permanent failure");
                },
                { attempts: 3 }
            )
        ).rejects.toThrow("Permanent failure");

        expect(attempts).toBe(3);
    });

    test("withRetry uses exponential backoff", async () => {
        const startTime = Date.now();
        let attempts = 0;

        try {
            await withRetry(
                async () => {
                    attempts++;
                    throw new Error("Fail");
                },
                { attempts: 3, backoff: 'exponential', initialDelay: 10 }
            );
        } catch {

        }

        const elapsed = Date.now() - startTime;

        expect(elapsed).toBeGreaterThanOrEqual(25);
        expect(attempts).toBe(3);
    });

    test("withRetry uses linear backoff", async () => {
        const startTime = Date.now();
        let attempts = 0;

        try {
            await withRetry(
                async () => {
                    attempts++;
                    throw new Error("Fail");
                },
                { attempts: 3, backoff: 'linear', initialDelay: 10 }
            );
        } catch {

        }

        const elapsed = Date.now() - startTime;

        expect(elapsed).toBeGreaterThanOrEqual(25);
        expect(attempts).toBe(3);
    });

    test("executeWithRetry helper works", async () => {
        let attempts = 0;
        const result = await executeWithRetry(
            async () => {
                attempts++;
                if (attempts < 2) throw new Error("Retry");
                return "done";
            },
            { attempts: 3 }
        );

        expect(result).toBe("done");
        expect(attempts).toBe(2);
    });
});

describe("Combined timeout and retry", () => {
    test("executeWithTimeoutAndRetry applies timeout to each attempt", async () => {
        let attempts = 0;

        await expect(
            executeWithTimeoutAndRetry(
                async () => {
                    attempts++;
                    await new Promise(resolve => setTimeout(resolve, 100));
                    return "success";
                },
                50, // 50ms timeout
                { attempts: 2 }
            )
        ).rejects.toThrow();

        // Should try twice and timeout both times
        expect(attempts).toBe(2);
    });

    test("executeWithTimeoutAndRetry succeeds if operation completes", async () => {
        let attempts = 0;
        const result = await executeWithTimeoutAndRetry(
            async () => {
                attempts++;
                if (attempts < 2) {
                    throw new Error("Retry");
                }
                return "success";
            },
            1000,
            { attempts: 3 }
        );

        expect(result).toBe("success");
        expect(attempts).toBe(2);
    });
});
