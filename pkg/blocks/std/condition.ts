
// Standard Block: Conditional Branching

import { Block, BlockHelpers } from "../../bunock/sdk/sdk.ts";

// Type definitions for input/output
export interface ConditionConfig {
    expression: string;      // JavaScript expression to evaluate (e.g., "input.value > 100")
}

export interface ConditionOutput {
    result: boolean;         // Evaluation result
    expression: string;      // Original expression for debugging
}

// Safely evaluate JavaScript expression

export function evaluateExpression(expression: string, context: unknown): boolean {
    try {

        // The expression has access to 'input' variable containing the context
        const evalFunction = new Function("input", `
            'use strict';
            return Boolean(${expression});
        `);

        const result = evalFunction(context);
        return Boolean(result);
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        throw new Error(`Expression evaluation failed: ${message}`);
    }
}

// Validate expression syntax without executing
export function validateExpression(expression: string): void {
    try {

        new Function("input", `'use strict'; return Boolean(${expression});`);
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        throw new Error(`Invalid expression syntax: ${message}`);
    }
}

/**
 * Condition Block - evaluates JavaScript expressions for workflow branching
 */
export class ConditionBlock extends Block<ConditionConfig, ConditionOutput> {
    validate(config: unknown): asserts config is ConditionConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "expression");

        // Validate expression syntax
        validateExpression((config as any).expression);
    }

    async execute(config: ConditionConfig, input?: unknown): Promise<ConditionOutput> {

        const context = input ?? {};

        // Evaluate expression
        const result = evaluateExpression(config.expression, context);

        return {
            result,
            expression: config.expression,
        };
    }

    // Route based on boolean result
    protected getOutputPort(result: ConditionOutput): string {
        return BlockHelpers.getBooleanPort(result.result);
    }
}

// Only run if this is the entry point
if (import.meta.main) {
    new ConditionBlock().run();
}
