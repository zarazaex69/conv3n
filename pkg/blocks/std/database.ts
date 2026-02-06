import { Block, BlockHelpers } from "#sdk";
import { Database } from "bun:sqlite";

const MAX_RESULT_ROWS = 10000;

type DatabaseOperation =
    | { type: "query"; sql: string; params?: unknown[] }
    | { type: "execute"; sql: string; params?: unknown[] }
    | { type: "transaction"; statements: Array<{ sql: string; params?: unknown[] }> };

interface DatabaseConfig {
    database: string;
    operation: DatabaseOperation;
}

type DatabaseOutput =
    | { operation: "query"; rows: unknown[]; rowCount: number }
    | { operation: "execute"; changes: number; lastInsertRowid: number }
    | { operation: "transaction"; statementsExecuted: number; changes: number };

export class DatabaseBlock extends Block<DatabaseConfig, DatabaseOutput> {
    private db: Database | null = null;

    validate(config: unknown): asserts config is DatabaseConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "database");
        BlockHelpers.assertField(config, "operation", "object");

        const op = config.operation as Record<string, unknown>;
        BlockHelpers.assertField(op, "type", "string");

        const validTypes = ["query", "execute", "transaction"];
        if (!validTypes.includes(op.type as string)) {
            throw new Error(`Invalid operation type: ${op.type}. Must be one of: ${validTypes.join(", ")}`);
        }

        if (op.type === "query" || op.type === "execute") {
            BlockHelpers.assertNonEmptyString(op, "sql");
            if ("params" in op && op.params !== undefined) {
                BlockHelpers.assertField(op, "params", "array");
            }
        }

        if (op.type === "transaction") {
            BlockHelpers.assertField(op, "statements", "array");
            const statements = op.statements as unknown[];
            if (statements.length === 0) {
                throw new Error("Transaction must have at least one statement");
            }
            for (const stmt of statements) {
                BlockHelpers.assertObject(stmt, "statement");
                BlockHelpers.assertNonEmptyString(stmt, "sql");
                if ("params" in stmt && stmt.params !== undefined) {
                    BlockHelpers.assertField(stmt, "params", "array");
                }
            }
        }
    }

    async execute(config: DatabaseConfig): Promise<DatabaseOutput> {
        try {
            this.db = new Database(config.database);
        } catch (error) {
            throw new Error(`Failed to open database: ${error instanceof Error ? error.message : String(error)}`);
        }

        try {
            const { operation } = config;

            switch (operation.type) {
                case "query":
                    return this.executeQuery(operation.sql, operation.params);
                case "execute":
                    return this.executeStatement(operation.sql, operation.params);
                case "transaction":
                    return this.executeTransaction(operation.statements);
                default:
                    throw new Error(`Unknown operation type: ${(operation as any).type}`);
            }
        } finally {
            if (this.db) {
                try {
                    this.db.close();
                } catch {
                }
            }
        }
    }

    private executeQuery(sql: string, params?: unknown[]): DatabaseOutput {
        if (!this.db) throw new Error("Database not initialized");

        try {
            const stmt = this.db.query(sql);
            const rows = params ? stmt.all(...params) : stmt.all();

            if (rows.length > MAX_RESULT_ROWS) {
                throw new Error(`Query returned ${rows.length} rows, exceeding maximum allowed (${MAX_RESULT_ROWS})`);
            }

            return { operation: "query", rows, rowCount: rows.length };
        } catch (error) {
            if (error instanceof Error && error.message.includes("exceeding maximum")) {
                throw error;
            }
            throw new Error(`SQL query failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private executeStatement(sql: string, params?: unknown[]): DatabaseOutput {
        if (!this.db) throw new Error("Database not initialized");

        try {
            const stmt = this.db.query(sql);
            const result = params ? stmt.run(...params) : stmt.run();

            return {
                operation: "execute",
                changes: result.changes,
                lastInsertRowid: Number(result.lastInsertRowid),
            };
        } catch (error) {
            throw new Error(`SQL execution failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private executeTransaction(statements: Array<{ sql: string; params?: unknown[] }>): DatabaseOutput {
        if (!this.db) throw new Error("Database not initialized");

        let totalChanges = 0;
        let statementsExecuted = 0;

        try {
            this.db.transaction(() => {
                for (const stmt of statements) {
                    const result = this.executeStatement(stmt.sql, stmt.params);
                    if (result.operation === "execute") {
                        totalChanges += result.changes;
                    }
                    statementsExecuted++;
                }
            })();

            return { operation: "transaction", statementsExecuted, changes: totalChanges };
        } catch (error) {
            throw new Error(`Transaction failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }
}

if (import.meta.main) {
    new DatabaseBlock().run();
}
