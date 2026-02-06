import { test, expect, describe } from "bun:test";
import { ExampleBlock } from "./example_block.ts";

describe("ExampleBlock", () => {
    test("validates config correctly", () => {
        const block = new ExampleBlock();

        expect(() => block.validate(null)).toThrow("Config must be an object");
        expect(() => block.validate({})).toThrow("message must be a non-empty string");
        expect(() => block.validate({ message: "" })).toThrow("message must be a non-empty string");
        expect(() => block.validate({ message: "test", multiplier: "invalid" })).toThrow("multiplier must be a number");

        expect(() => block.validate({ message: "test" })).not.toThrow();
        expect(() => block.validate({ message: "test", multiplier: 3 })).not.toThrow();
    });

    test("executes with default multiplier", async () => {
        const block = new ExampleBlock();
        const result = await block.execute({ message: "hello" });

        expect(result.result).toBe("hello");
        expect(result.count).toBe(5);
    });

    test("executes with custom multiplier", async () => {
        const block = new ExampleBlock();
        const result = await block.execute({ message: "hi", multiplier: 3 });

        expect(result.result).toBe("hihihi");
        expect(result.count).toBe(6);
    });

    test("handles input parameter", async () => {
        const block = new ExampleBlock();
        const result = await block.execute(
            { message: "test" },
            { someData: "ignored" }
        );

        expect(result.result).toBe("test");
    });
});
