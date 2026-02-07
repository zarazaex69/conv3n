import { describe, test, expect } from "bun:test";

describe("CounterBlock", () => {
    test("should increment counter", async () => {
        const proc = Bun.spawn(["bun", "run", "pkg/blocks/std/counter.ts"], {
            stdin: "pipe",
            stdout: "pipe",
        });

        const input = {
            config: {
                counterName: "test_counter",
                increment: 5,
                scope: "execution",
            },
            input: {
                context: {
                    variables: {
                        test_counter: 10,
                    },
                },
            },
        };

        proc.stdin.write(JSON.stringify(input));
        proc.stdin.end();

        const output = await new Response(proc.stdout).json() as any;

        expect(output.data.currentValue).toBe(15);
        expect(output.data.previousValue).toBe(10);
        expect(output.port).toBe("default");
        expect(output.variables).toHaveLength(1);
        expect(output.variables[0].action).toBe("set");
        expect(output.variables[0].name).toBe("test_counter");
        expect(output.variables[0].value).toBe(15);
    });

    test("should start from zero if no previous value", async () => {
        const proc = Bun.spawn(["bun", "run", "pkg/blocks/std/counter.ts"], {
            stdin: "pipe",
            stdout: "pipe",
        });

        const input = {
            config: {
                counterName: "new_counter",
            },
        };

        proc.stdin.write(JSON.stringify(input));
        proc.stdin.end();

        const output = await new Response(proc.stdout).json() as any;

        expect(output.data.currentValue).toBe(1);
        expect(output.data.previousValue).toBe(0);
    });

    test("should support global scope with TTL", async () => {
        const proc = Bun.spawn(["bun", "run", "pkg/blocks/std/counter.ts"], {
            stdin: "pipe",
            stdout: "pipe",
        });

        const input = {
            config: {
                counterName: "global_counter",
                increment: 1,
                scope: "global",
                ttlSeconds: 60,
            },
        };

        proc.stdin.write(JSON.stringify(input));
        proc.stdin.end();

        const output = await new Response(proc.stdout).json() as any;

        expect(output.variables[0].options.scope).toBe("global");
        expect(output.variables[0].options.ttlSeconds).toBe(60);
    });
});
