
// SDK for building type-safe blocks in the CONV3N workflow engine

/**
 * Standard input structure for all blocks
 * @template TConfig - Type of the block configuration
 */
export interface BlockInput<TConfig = unknown> {
    config: TConfig;
    input?: unknown;
    context?: ExecutionContextData;
}

export interface ExecutionContextData {
    workflowId: string;
    executionId: string;
    variables?: Record<string, unknown>;
}

/**
 * Standard output structure for all blocks
 * @template TData - Type of the output data
 */
export interface BlockOutput<TData = unknown> {
    data: TData;
    port: string;
    variables?: VariableCommand[];
}

import type { VariableCommand } from "./variables.ts";

/**
 * Error details structure
 */
export interface ErrorDetails {
    message: string;
    type: string;
    stack?: string;
}

export class BlockExecutionError extends Error {
    readonly type = "BlockExecutionError";
    constructor(message: string, public readonly cause?: unknown) {
        super(message);
        this.name = "BlockExecutionError";
    }
}

export class ValidationError extends Error {
    readonly type = "ValidationError";
    constructor(message: string, public readonly field?: string) {
        super(message);
        this.name = "ValidationError";
    }
}

export type Result<T, E = Error> = 
    | { ok: true; value: T }
    | { ok: false; error: E };

export function Ok<T>(value: T): Result<T, never> {
    return { ok: true, value };
}

export function Err<E>(error: E): Result<never, E> {
    return { ok: false, error };
}

/**
 * Base abstract class for building type-safe blocks
 * Handles stdin/stdout communication protocol automatically
 * 
 * @template TConfig - Type of the block configuration
 * @template TOutput - Type of the block output data
 * 
 * @example
 * ```typescript
 * class MyBlock extends Block<MyConfig, MyOutput> {
 *   validate(config: unknown): asserts config is MyConfig {
 *     if (!config || typeof config !== 'object') throw new Error('Invalid config');
 *   }
 *   
 *   async execute(config: MyConfig, input?: unknown): Promise<MyOutput> {
 *     return { result: 'success' };
 *   }
 * }
 * 
 * if (import.meta.main) {
 *   new MyBlock().run();
 * }
 * ```
 */
export abstract class Block<TConfig, TOutput> {
    protected variables = new VariableManager();

    abstract validate(config: unknown): asserts config is TConfig;

    /**
     * Execute the block logic
     * @param config - Validated configuration
     * @param input - Optional input data from previous blocks or variables
     * @returns Output data to be passed to next blocks
     */
    abstract execute(config: TConfig, input?: unknown): Promise<TOutput>;

    /**
     * Main entry point - orchestrates the block execution lifecycle
     * 1. Read input from stdin
     * 2. Validate configuration
     * 3. Execute block logic
     * 4. Write output to stdout
     */
    async run(): Promise<void> {
        try {

            const input = await this.readInput();

            // Validate configuration
            this.validate(input.config);

            // Execute block logic
            const result = await this.execute(input.config, input.input);

            // Determine output port (default implementation)
            const port = this.getOutputPort(result);

            // Write success output
            await this.writeOutput(result, port);

        } catch (error) {

            await this.handleError(error);
        }
    }

    /**
     * Read and parse input from stdin
     * Override this method if you need custom input parsing
     */
    protected async readInput(): Promise<BlockInput<TConfig>> {
        return await Bun.stdin.json();
    }

    /**
     * Write output to stdout in standard format
     * @param data - Output data
     * @param port - Output port name (for routing in workflow graph)
     */
    protected async writeOutput(data: TOutput, port: string): Promise<void> {
        const output: BlockOutput<TOutput> = { 
            data, 
            port,
            variables: this.variables.getCommands(),
        };
        await Bun.write(Bun.stdout, JSON.stringify(output));
    }

    /**
     * Write error to stdout and exit with error code
     * @param error - Error object or message
     * @param port - Error port name (default: "error")
     */
    protected async writeError(error: unknown, port = "error"): Promise<void> {
        const errorDetails = this.formatError(error);
        const output: BlockOutput<{ error: ErrorDetails }> = {
            data: { error: errorDetails },
            port,
        };
        await Bun.write(Bun.stdout, JSON.stringify(output));
        process.exit(1);
    }

    /**
     * Determine output port based on result
     * Override this method to implement custom routing logic
     * Default implementation returns "default"
     */
    protected getOutputPort(result: TOutput): string {
        return "default";
    }

    /**
     * Handle errors during block execution
     * Override this method to implement custom error handling
     */
    protected async handleError(error: unknown): Promise<void> {
        await this.writeError(error);
    }

    /**
     * Format error object into standard structure
     */
    protected formatError(error: unknown): ErrorDetails {
        if (error instanceof Error) {
            return {
                message: error.message,
                type: error.name || "Error",
                stack: error.stack,
            };
        }
        return {
            message: String(error),
            type: "UnknownError",
        };
    }
}

/**
 * Utility helpers for building blocks
 * Provides common functions for creating results, validating inputs, etc.
 */
export class BlockHelpers {
    static createSuccessResult<T>(data: T, port = "default"): BlockOutput<T> {
        return { data, port };
    }

    /**
     * Create an error result with specified message and type
     */
    static createErrorResult(message: string, type = "Error", stack?: string): BlockOutput<{ error: ErrorDetails }> {
        return {
            data: {
                error: { message, type, stack },
            },
            port: "error",
        };
    }

    /**
     * Validate that a value is a non-null object
     * @throws Error if validation fails
     */
    static assertObject(value: unknown, fieldName = "config"): asserts value is Record<string, unknown> {
        if (!value || typeof value !== "object" || Array.isArray(value)) {
            throw new Error(`${fieldName} must be a non-null object`);
        }
    }

    /**
     * Validate that a field exists and is of specified type
     * @throws Error if validation fails
     */
    static assertField<T>(
        obj: Record<string, unknown>,
        field: string,
        type: "string" | "number" | "boolean" | "object" | "array"
    ): asserts obj is Record<string, T> {
        if (!(field in obj)) {
            throw new Error(`Missing required field: ${field}`);
        }

        const value = obj[field];

        if (type === "array") {
            if (!Array.isArray(value)) {
                throw new Error(`Field '${field}' must be an array`);
            }
        } else if (type === "object") {
            if (!value || typeof value !== "object" || Array.isArray(value)) {
                throw new Error(`Field '${field}' must be an object`);
            }
        } else {
            if (typeof value !== type) {
                throw new Error(`Field '${field}' must be of type ${type}`);
            }
        }
    }

    /**
     * Validate that a string field is not empty
     * @throws Error if validation fails
     */
    static assertNonEmptyString(obj: Record<string, unknown>, field: string): void {
        this.assertField(obj, field, "string");
        if ((obj[field] as string).trim() === "") {
            throw new Error(`Field '${field}' must not be empty`);
        }
    }

    /**
     * Safely parse JSON with fallback to raw string
     */
    static parseJSON(text: string): unknown {
        try {
            return JSON.parse(text);
        } catch {
            return text;
        }
    }

    /**
     * Determine HTTP status-based routing port
     * Useful for HTTP request blocks
     */
    static getHttpPort(status: number): string {
        if (status >= 200 && status < 300) {
            return "success";
        } else if (status >= 400 && status < 500) {
            return "client_error";
        } else if (status >= 500) {
            return "server_error";
        }
        return "default";
    }

    /**
     * Determine boolean-based routing port
     * Useful for condition blocks
     */
    static getBooleanPort(value: boolean): string {
        return value ? "true" : "false";
    }
}

/**
 * Type guard to check if a value is a valid BlockInput
 */
export function isBlockInput(value: unknown): value is BlockInput {
    return (
        typeof value === "object" &&
        value !== null &&
        "config" in value
    );
}

/**
 * Type guard to check if a value is a valid BlockOutput
 */
export function isBlockOutput(value: unknown): value is BlockOutput {
    return (
        typeof value === "object" &&
        value !== null &&
        "data" in value &&
        "port" in value &&
        typeof (value as BlockOutput).port === "string"
    );
}

export * from "./decorators.ts";
export * from "./schema.ts";
export * from "./trigger.ts";
export * from "./variables.ts";

import { VariableManager } from "./variables.ts";
