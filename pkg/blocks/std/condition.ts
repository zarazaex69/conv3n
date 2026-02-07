
import { Block, BlockHelpers } from "../../bunock/sdk/sdk.ts";

export interface ConditionConfig {
    expression: string;
}

export interface ConditionOutput {
    result: boolean;
    expression: string;
}

export function evaluateExpression(expression: string, context: unknown): boolean {
    validateExpressionSafety(expression);

    try {
        const safeContext = sanitizeContext(context);

        const evalFunction = new Function("input", `
            'use strict';
            return Boolean(${expression});
        `);

        const result = evalFunction(safeContext);
        return Boolean(result);
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        throw new Error(`Expression evaluation failed: ${message}`);
    }
}

export function validateExpression(expression: string): void {
    validateExpressionSafety(expression);

    try {
        new Function("input", `'use strict'; return Boolean(${expression});`);
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        throw new Error(`Invalid expression syntax: ${message}`);
    }
}

function validateExpressionSafety(expression: string): void {
    const dangerousPatterns = [
        /require\s*\(/,
        /import\s+/,
        /process\./,
        /global\./,
        /Function\s*\(/,
        /eval\s*\(/,
        /setTimeout\s*\(/,
        /setInterval\s*\(/,
        /__dirname/,
        /__filename/,
        /Bun\./,
    ];

    for (const pattern of dangerousPatterns) {
        if (pattern.test(expression)) {
            throw new Error(`Expression contains forbidden pattern: ${pattern.source}`);
        }
    }

    if (expression.length > 1000) {
        throw new Error("Expression too long (max 1000 characters)");
    }
}

function sanitizeContext(context: unknown): unknown {
    if (typeof context !== "object" || context === null) {
        return context;
    }

    const sanitized: Record<string, unknown> = {};
    const obj = context as Record<string, unknown>;

    for (const [key, value] of Object.entries(obj)) {
        if (typeof value === "function") {
            continue;
        }
        sanitized[key] = value;
    }

    return sanitized;
}

export class ConditionBlock extends Block<ConditionConfig, ConditionOutput> {
    validate(config: unknown): asserts config is ConditionConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "expression");

        validateExpression((config as any).expression);
    }

    async execute(config: ConditionConfig, input?: unknown): Promise<ConditionOutput> {
        const context = input ?? {};

        const result = evaluateExpression(config.expression, context);

        return {
            result,
            expression: config.expression,
        };
    }

    protected getOutputPort(result: ConditionOutput): string {
        return BlockHelpers.getBooleanPort(result.result);
    }
}

if (import.meta.main) {
    new ConditionBlock().run();
}
