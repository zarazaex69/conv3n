import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { FileBlock } from "./file";
import { mkdirSync, rmSync } from "fs";
import { join } from "path";

const TEST_DIR = "/tmp/conv3n_file_tests";

describe("File Block", () => {
    beforeEach(() => {
        try {
            mkdirSync(TEST_DIR, { recursive: true });
        } catch (e) {
        }
    });

    afterEach(() => {
        try {
            rmSync(TEST_DIR, { recursive: true, force: true });
        } catch (e) {
        }
    });

    describe("validate", () => {
        test("should pass for valid read config", () => {
            const block = new FileBlock();
            const config = {
                path: "/tmp/test.txt",
                operation: { type: "read" as const },
            };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should pass for valid write config", () => {
            const block = new FileBlock();
            const config = {
                path: "/tmp/test.txt",
                operation: { type: "write" as const, content: "Hello" },
            };
            expect(() => block.validate(config)).not.toThrow();
        });

        test("should throw error when path is missing", () => {
            const block = new FileBlock();
            const config = {
                operation: { type: "read" as const },
            };
            expect(() => block.validate(config)).toThrow();
        });

        test("should throw error for invalid operation type", () => {
            const block = new FileBlock();
            const config = {
                path: "/tmp/test.txt",
                operation: { type: "invalid" },
            };
            expect(() => block.validate(config)).toThrow("Invalid operation type");
        });
    });

    describe("execute", () => {
        test("should read text file", async () => {
            const block = new FileBlock();
            const path = join(TEST_DIR, "test.txt");
            await Bun.write(path, "Hello, World!");

            const result = await block.execute({
                path,
                operation: { type: "read", format: "text" },
            });

            expect(result.operation).toBe("read");
            if (result.operation === "read") {
                expect(result.data).toBe("Hello, World!");
                expect(result.size).toBe(13);
            }
        });

        test("should write text content", async () => {
            const block = new FileBlock();
            const path = join(TEST_DIR, "write_text.txt");

            const result = await block.execute({
                path,
                operation: { type: "write", content: "Hello from Bun!" },
            });

            expect(result.operation).toBe("write");
            if (result.operation === "write") {
                expect(result.path).toBe(path);
                expect(result.bytesWritten).toBeGreaterThan(0);
            }

            const file = Bun.file(path);
            const readContent = await file.text();
            expect(readContent).toBe("Hello from Bun!");
        });

        test("should delete existing file", async () => {
            const block = new FileBlock();
            const path = join(TEST_DIR, "delete_me.txt");
            await Bun.write(path, "To be deleted");

            const result = await block.execute({
                path,
                operation: { type: "delete" },
            });

            expect(result.operation).toBe("delete");
            if (result.operation === "delete") {
                expect(result.deleted).toBe(true);
            }

            const file = Bun.file(path);
            expect(await file.exists()).toBe(false);
        });

        test("should check file existence", async () => {
            const block = new FileBlock();
            const path = join(TEST_DIR, "exists.txt");
            await Bun.write(path, "I exist");

            const result = await block.execute({
                path,
                operation: { type: "exists" },
            });

            expect(result.operation).toBe("exists");
            if (result.operation === "exists") {
                expect(result.exists).toBe(true);
            }
        });

        test("should throw error for non-existent file read", async () => {
            const block = new FileBlock();
            const path = join(TEST_DIR, "nonexistent.txt");

            await expect(
                block.execute({
                    path,
                    operation: { type: "read" },
                })
            ).rejects.toThrow("File not found");
        });
    });
});
