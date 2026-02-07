
// Custom Code Block: Execute user-provided TypeScript/JavaScript

// Define type-safe input/output interfaces
interface CustomCodeInput {
    config: {
        code: string;        // User-provided code as a string
        input?: unknown;     // Optional input data resolved from variables
    };
    input?: unknown;         // Optional input data from previous blocks
}

interface CustomCodeOutput {
    success: boolean;
    data?: unknown;
    error?: {
        message: string;
        stack?: string;
        type: string;
    };
    executionTime: number;
}

interface BlockResult {
    data: CustomCodeOutput;
    port: string;
}

// Helper to create error result
function createErrorResult(
    message: string,
    type: string,
    stack: string | undefined,
    executionTime: number
): BlockResult {
    return {
        data: {
            success: false,
            error: { message, type, stack },
            executionTime,
        },
        port: "error",
    };
}

async function main(): Promise<void> {
    const startTime = performance.now();

    try {

        const input: CustomCodeInput = await Bun.stdin.json();

        if (!input.config || !input.config.code) {
            throw new Error("Missing required config: code");
        }

        const userCode = input.config.code;

        // Determine the data to pass to user function

        // Otherwise, use the input field from the payload
        const userData = input.config.input !== undefined
            ? input.config.input
            : (input.input ?? {});

        try {
            const transpiler = new Bun.Transpiler({ loader: "ts" });
            transpiler.transformSync(userCode);
        } catch (syntaxError) {
            const endTime = performance.now();
            const err = syntaxError instanceof Error ? syntaxError : new Error(String(syntaxError));
            const result = createErrorResult(
                `Syntax Error: ${err.message}`,
                "SyntaxError",
                err.stack,
                endTime - startTime
            );
            await Bun.write(Bun.stdout, JSON.stringify(result));
            return;
        }


        // Example: export default async (input) => { return { result: input.value * 2 }; }

        let userFunction: (input: unknown) => Promise<unknown>;

        try {

            const moduleCode = userCode.includes("export default")
                ? userCode
                : `export default async (input) => { ${userCode} }`;

            // Use dynamic import with data URL to execute the code
            const dataUrl = `data:text/typescript;base64,${btoa(moduleCode)}`;
            const module = await import(dataUrl);
            userFunction = module.default;

            if (typeof userFunction !== "function") {
                throw new Error("User code must export a default function");
            }
        } catch (importError) {
            const endTime = performance.now();
            const err = importError instanceof Error ? importError : new Error(String(importError));
            const result = createErrorResult(
                `Import Error: ${err.message}`,
                "ImportError",
                err.stack,
                endTime - startTime
            );
            await Bun.write(Bun.stdout, JSON.stringify(result));
            return;
        }

        let executionResult: unknown;

        try {
            executionResult = await userFunction(userData);
        } catch (runtimeError) {
            const endTime = performance.now();
            const err = runtimeError instanceof Error ? runtimeError : new Error(String(runtimeError));
            const result = createErrorResult(
                `Runtime Error: ${err.message}`,
                err.name || "RuntimeError",
                err.stack,
                endTime - startTime
            );
            await Bun.write(Bun.stdout, JSON.stringify(result));
            return;
        }

        const endTime = performance.now();
        const result: BlockResult = {
            data: {
                success: true,
                data: executionResult,
                executionTime: endTime - startTime,
            },
            port: "default",
        };

        await Bun.write(Bun.stdout, JSON.stringify(result));

    } catch (error) {

        const endTime = performance.now();
        const err = error instanceof Error ? error : new Error(String(error));
        console.error(`Custom Code Block Failed: ${err.message}`);
        const result = createErrorResult(
            err.message || "Unknown error",
            "UnexpectedError",
            err.stack,
            endTime - startTime
        );
        await Bun.write(Bun.stdout, JSON.stringify(result));
        process.exit(1);
    }
}

main();
