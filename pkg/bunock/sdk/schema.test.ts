import { describe, test, expect } from "bun:test";
import {
    validateSchema,
    createSchemaValidator,
    CommonSchemas,
    type ConfigSchema,
} from "./schema.ts";

describe("Schema validation", () => {
    test("validates required string field", () => {
        const schema: ConfigSchema = {
            name: { type: 'string', required: true },
        };

        expect(() => validateSchema({ name: "test" }, schema)).not.toThrow();
        expect(() => validateSchema({}, schema)).toThrow("Missing required field: name");
    });

    test("validates field types", () => {
        const schema: ConfigSchema = {
            count: { type: 'number' },
            enabled: { type: 'boolean' },
            items: { type: 'array' },
            config: { type: 'object' },
        };

        const valid = {
            count: 42,
            enabled: true,
            items: [1, 2, 3],
            config: { key: "value" },
        };

        expect(() => validateSchema(valid, schema)).not.toThrow();

        // Wrong types
        expect(() => validateSchema({ count: "not a number" }, schema))
            .toThrow("must be of type number");

        expect(() => validateSchema({ enabled: "not a boolean" }, schema))
            .toThrow("must be of type boolean");

        expect(() => validateSchema({ items: "not an array" }, schema))
            .toThrow("must be an array");

        expect(() => validateSchema({ config: [1, 2] }, schema))
            .toThrow("must be an object");
    });

    test("applies default values", () => {
        const schema: ConfigSchema = {
            timeout: { type: 'number', default: 5000 },
            method: { type: 'string', default: 'GET' },
        };

        const config = {};
        validateSchema(config, schema);

        expect((config as any).timeout).toBe(5000);
        expect((config as any).method).toBe('GET');
    });

    test("runs custom validation", () => {
        const schema: ConfigSchema = {
            port: {
                type: 'number',
                validate: (v) => typeof v === 'number' && v >= 1 && v <= 65535,
                errorMessage: 'Port must be between 1 and 65535',
            },
        };

        expect(() => validateSchema({ port: 8080 }, schema)).not.toThrow();
        expect(() => validateSchema({ port: 70000 }, schema))
            .toThrow("Port must be between 1 and 65535");
    });

    test("allows optional fields", () => {
        const schema: ConfigSchema = {
            required: { type: 'string', required: true },
            optional: { type: 'string' },
        };

        expect(() => validateSchema({ required: "test" }, schema)).not.toThrow();
        expect(() => validateSchema({}, schema)).toThrow("Missing required field: required");
    });
});

describe("createSchemaValidator", () => {
    test("creates type-safe validator function", () => {
        interface MyConfig {
            url: string;
            timeout: number;
        }

        const schema: ConfigSchema = {
            url: { type: 'string', required: true },
            timeout: { type: 'number', default: 5000 },
        };

        const validate: (config: unknown) => asserts config is MyConfig = createSchemaValidator<MyConfig>(schema);

        const config: unknown = { url: "https://example.com" };
        validate(config);

        expect(config.url).toBe("https://example.com");
        expect(config.timeout).toBe(5000);
    });
});

describe("CommonSchemas", () => {
    test("url schema validates URLs", () => {
        const schema: ConfigSchema = {
            endpoint: CommonSchemas.url,
        };

        expect(() => validateSchema({ endpoint: "https://example.com" }, schema)).not.toThrow();
        expect(() => validateSchema({ endpoint: "not a url" }, schema))
            .toThrow("Must be a valid URL");
    });

    test("positiveNumber schema validates positive numbers", () => {
        const schema: ConfigSchema = {
            count: CommonSchemas.positiveNumber,
        };

        expect(() => validateSchema({ count: 42 }, schema)).not.toThrow();
        expect(() => validateSchema({ count: -5 }, schema))
            .toThrow("Must be a positive number");
        expect(() => validateSchema({ count: 0 }, schema))
            .toThrow("Must be a positive number");
    });

    test("nonEmptyString schema validates non-empty strings", () => {
        const schema: ConfigSchema = {
            name: CommonSchemas.nonEmptyString,
        };

        expect(() => validateSchema({ name: "test" }, schema)).not.toThrow();
        expect(() => validateSchema({ name: "" }, schema))
            .toThrow("Must be a non-empty string");
        expect(() => validateSchema({ name: "   " }, schema))
            .toThrow("Must be a non-empty string");
    });

    test("email schema validates email addresses", () => {
        const schema: ConfigSchema = {
            email: CommonSchemas.email,
        };

        expect(() => validateSchema({ email: "test@example.com" }, schema)).not.toThrow();
        expect(() => validateSchema({ email: "not an email" }, schema))
            .toThrow("Must be a valid email address");
    });

    test("port schema validates port numbers", () => {
        const schema: ConfigSchema = {
            port: CommonSchemas.port,
        };

        expect(() => validateSchema({ port: 8080 }, schema)).not.toThrow();
        expect(() => validateSchema({ port: 0 }, schema))
            .toThrow("Must be a valid port number");
        expect(() => validateSchema({ port: 70000 }, schema))
            .toThrow("Must be a valid port number");
    });
});
