import { Block, BlockHelpers } from "#sdk";

type FileOperation =
    | { type: "read"; format?: "text" | "json" | "bytes" }
    | { type: "write"; content: string | object }
    | { type: "delete" }
    | { type: "exists" };

interface FileConfig {
    path: string;
    operation: FileOperation;
    allowedPaths?: string[];
}

type FileOutput =
    | { operation: "read"; data: string | object | Uint8Array; size: number }
    | { operation: "write"; path: string; bytesWritten: number }
    | { operation: "delete"; path: string; deleted: boolean }
    | { operation: "exists"; path: string; exists: boolean };

const FORBIDDEN_PATHS = [
    "/etc/passwd",
    "/etc/shadow",
    "/etc/hosts",
    "~/.ssh",
    "~/.aws",
    "/proc",
    "/sys",
];

const MAX_FILE_SIZE = 50 * 1024 * 1024;

export class FileBlock extends Block<FileConfig, FileOutput> {
    validate(config: unknown): asserts config is FileConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "path");
        BlockHelpers.assertField(config, "operation", "object");

        this.validatePath(config.path as string, config.allowedPaths as string[] | undefined);

        const op = config.operation as Record<string, unknown>;
        BlockHelpers.assertField(op, "type", "string");

        const validTypes = ["read", "write", "delete", "exists"];
        if (!validTypes.includes(op.type as string)) {
            throw new Error(`Invalid operation type: ${op.type}. Must be one of: ${validTypes.join(", ")}`);
        }

        if (op.type === "read" && "format" in op && op.format !== undefined) {
            const validFormats = ["text", "json", "bytes"];
            if (!validFormats.includes(op.format as string)) {
                throw new Error(`Invalid read format: ${op.format}. Must be one of: ${validFormats.join(", ")}`);
            }
        }

        if (op.type === "write" && !("content" in op)) {
            throw new Error("Write operation requires 'content' field");
        }
    }

    async execute(config: FileConfig): Promise<FileOutput> {
        const { path, operation } = config;

        this.validatePath(path, config.allowedPaths);

        switch (operation.type) {
            case "read":
                return await this.executeRead(path, operation.format || "text");
            case "write":
                return await this.executeWrite(path, operation.content);
            case "delete":
                return await this.executeDelete(path);
            case "exists":
                return await this.executeExists(path);
            default:
                throw new Error(`Unknown operation type: ${(operation as any).type}`);
        }
    }

    private validatePath(path: string, allowedPaths?: string[]): void {
        const normalizedPath = this.normalizePath(path);

        for (const forbidden of FORBIDDEN_PATHS) {
            if (normalizedPath.startsWith(forbidden) || normalizedPath.includes(forbidden)) {
                throw new Error(`Access to path '${path}' is forbidden`);
            }
        }

        if (normalizedPath.includes("..")) {
            throw new Error("Path traversal detected: '..' is not allowed");
        }

        if (allowedPaths && allowedPaths.length > 0) {
            const isAllowed = allowedPaths.some(allowed => {
                const normalizedAllowed = this.normalizePath(allowed);
                return normalizedPath.startsWith(normalizedAllowed);
            });

            if (!isAllowed) {
                throw new Error(`Path '${path}' is not in allowed paths list`);
            }
        }
    }

    private normalizePath(path: string): string {
        return path.replace(/\\/g, "/").replace(/\/+/g, "/");
    }

    private async executeRead(path: string, format: "text" | "json" | "bytes"): Promise<FileOutput> {
        const file = Bun.file(path);

        const exists = await file.exists();
        if (!exists) {
            throw new Error(`File not found: ${path}`);
        }

        const size = file.size;

        if (size > MAX_FILE_SIZE) {
            throw new Error(`File size ${size} exceeds maximum allowed ${MAX_FILE_SIZE} bytes`);
        }

        try {
            let data: string | object | Uint8Array;
            switch (format) {
                case "text":
                    data = await file.text();
                    break;
                case "json":
                    data = await file.json();
                    break;
                case "bytes":
                    data = await file.bytes();
                    break;
            }

            return { operation: "read", data, size };
        } catch (error) {
            if (format === "json") {
                throw new Error(`Failed to parse JSON from file: ${error instanceof Error ? error.message : String(error)}`);
            }
            throw new Error(`Failed to read file: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private async executeWrite(path: string, content: string | object): Promise<FileOutput> {
        try {
            const writeContent = typeof content === "object" ? JSON.stringify(content, null, 2) : content;

            const bytesWritten = await Bun.write(path, writeContent);

            return { operation: "write", path, bytesWritten };
        } catch (error) {
            throw new Error(`Failed to write file: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private async executeDelete(path: string): Promise<FileOutput> {
        try {
            const file = Bun.file(path);

            const exists = await file.exists();
            if (!exists) {
                throw new Error(`File not found: ${path}`);
            }

            await file.delete();

            return { operation: "delete", path, deleted: true };
        } catch (error) {
            if (error instanceof Error && error.message.includes("File not found")) {
                throw error;
            }
            throw new Error(`Failed to delete file: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private async executeExists(path: string): Promise<FileOutput> {
        try {
            const file = Bun.file(path);
            const exists = await file.exists();

            return { operation: "exists", path, exists };
        } catch (error) {
            throw new Error(`Failed to check file existence: ${error instanceof Error ? error.message : String(error)}`);
        }
    }
}

if (import.meta.main) {
    new FileBlock().run();
}
