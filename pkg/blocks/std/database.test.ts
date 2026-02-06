import { describe, test, expect } from "bun:test";
import { DatabaseBlock } from "./database";

describe("Database Block", () => {
    describe("validate", () => {
        test("should pass for valid query config", () => {
            const block = new DatabaseBlock();
            const config = {
                database: ":memory:",
                operation: { type: "query" as const, sql: "SELECT 1" },
            };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should pass for valid execute config", () => {
            const block = new DatabaseBlock();
            const config = {
                database: ":memory:",
                operation: { type: "execute" as const, sql: "INSERT INTO users VALUES (1)" },
            };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should throw error when database is missing", () => {
            const block = new DatabaseBlock();
            const config = {
                operation: { type: "query" as const, sql: "SELECT 1" },
            };
            expect(() => block.validate(config)).toThrow();
        });

        test("should throw error for invalid operation type", () => {
            const block = new DatabaseBlock();
            const config = {
                database: ":memory:",
                operation: { type: "invalid", sql: "SELECT 1" },
            };
            expect(() => block.validate(config)).toThrow("Invalid operation type");
        });
    });

    describe("execute", () => {
        test("should execute simple SELECT query", async () => {
            const block = new DatabaseBlock();

            const result = await block.execute({
                database: ":memory:",
                operation: { type: "query", sql: "SELECT 1 as value" },
            });

            expect(result.operation).toBe("query");
            if (result.operation === "query") {
                expect(result.rowCount).toBeGreaterThanOrEqual(0);
            }
        });

        test("should execute INSERT statement", async () => {
            const block = new DatabaseBlock();

            const result = await block.execute({
                database: ":memory:",
                operation: {
                    type: "execute",
                    sql: "CREATE TABLE test (id INTEGER); INSERT INTO test VALUES (1)",
                },
            });

            expect(result.operation).toBe("execute");
        });

        test("should execute transaction", async () => {
            const block = new DatabaseBlock();

            const result = await block.execute({
                database: ":memory:",
                operation: {
                    type: "transaction",
                    statements: [
                        { sql: "CREATE TABLE test (id INTEGER)" },
                        { sql: "INSERT INTO test VALUES (1)" },
                    ],
                },
            });

            expect(result.operation).toBe("transaction");
            if (result.operation === "transaction") {
                expect(result.statementsExecuted).toBe(2);
            }
        });
    });
});
