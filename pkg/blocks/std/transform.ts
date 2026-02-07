import { Block, BlockHelpers } from "#sdk";
import { query as jsonpathQuery } from "jsonpath-rfc9535";

type TransformOperation =
    | { type: "pick"; fields: string[] }
    | { type: "rename"; mapping: Record<string, string> }
    | { type: "map"; expression: string }
    | { type: "jsonpath"; query: string };

interface TransformConfig {
    input?: unknown;
    operations: TransformOperation[];
}

interface TransformOutput {
    data: unknown;
    operationsApplied: number;
}

export class TransformBlock extends Block<TransformConfig, TransformOutput> {
    validate(config: unknown): asserts config is TransformConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "operations", "array");

        const operations = config.operations as unknown[];
        if (operations.length === 0) {
            throw new Error("At least one operation is required");
        }

        for (const op of operations) {
            BlockHelpers.assertObject(op, "operation");
            BlockHelpers.assertField(op, "type", "string");

            const opType = op.type as string;
            switch (opType) {
                case "pick":
                    BlockHelpers.assertField(op, "fields", "array");
                    break;
                case "rename":
                    BlockHelpers.assertField(op, "mapping", "object");
                    break;
                case "map":
                    BlockHelpers.assertField(op, "expression", "string");
                    break;
                case "jsonpath":
                    BlockHelpers.assertField(op, "query", "string");
                    break;
                default:
                    throw new Error(`Unknown operation type: ${opType}`);
            }
        }
    }

    async execute(config: TransformConfig, input?: unknown): Promise<TransformOutput> {
        let data: unknown = config.input !== undefined ? config.input : (input ?? {});

        let operationsApplied = 0;
        for (const operation of config.operations) {
            switch (operation.type) {
                case "pick":
                    data = this.applyPick(data, operation.fields);
                    break;
                case "rename":
                    data = this.applyRename(data, operation.mapping);
                    break;
                case "map":
                    data = this.applyMap(data, operation.expression);
                    break;
                case "jsonpath":
                    data = this.applyJSONPath(data, operation.query);
                    break;
            }
            operationsApplied++;
        }

        return { data, operationsApplied };
    }

    private applyPick(data: unknown, fields: string[]): unknown {
        if (typeof data !== "object" || data === null) {
            throw new Error("pick operation requires an object");
        }

        const result: Record<string, unknown> = {};
        const obj = data as Record<string, unknown>;
        for (const field of fields) {
            if (field in obj) {
                result[field] = obj[field];
            }
        }
        return result;
    }

    private applyRename(data: unknown, mapping: Record<string, string>): unknown {
        if (typeof data !== "object" || data === null) {
            throw new Error("rename operation requires an object");
        }

        const result: Record<string, unknown> = {};
        const obj = data as Record<string, unknown>;
        for (const [key, value] of Object.entries(obj)) {
            const newKey = mapping[key] || key;
            result[newKey] = value;
        }
        return result;
    }

    private applyMap(data: unknown, expression: string): unknown {
        this.validateMapExpression(expression);

        try {
            const mapFn = new Function("data", `'use strict'; return (${expression});`);
            return mapFn(data);
        } catch (error) {
            throw new Error(`Map expression failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private validateMapExpression(expression: string): void {
        const dangerousPatterns = [
            /require\s*\(/,
            /import\s+/,
            /process\./,
            /global\./,
            /Function\s*\(/,
            /eval\s*\(/,
            /Bun\./,
        ];

        for (const pattern of dangerousPatterns) {
            if (pattern.test(expression)) {
                throw new Error(`Expression contains forbidden pattern: ${pattern.source}`);
            }
        }

        if (expression.length > 2000) {
            throw new Error("Expression too long (max 2000 characters)");
        }
    }

    private applyJSONPath(data: unknown, queryString: string): unknown {
        try {
            const result = jsonpathQuery(data as any, queryString);

            if (Array.isArray(result) && result.length === 1) {
                return result[0];
            }

            return result;
        } catch (error) {
            throw new Error(`JSONPath query failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }
}

if (import.meta.main) {
    new TransformBlock().run();
}
