export type FieldType = 'string' | 'number' | 'boolean' | 'object' | 'array';

export interface FieldSchema {
    type: FieldType;
    required?: boolean;
    default?: unknown;
    validate?: (value: unknown) => boolean;
    errorMessage?: string;
}

export type ConfigSchema = Record<string, FieldSchema>;

export function validateSchema(config: unknown, schema: ConfigSchema): void {
    if (!config || typeof config !== 'object' || Array.isArray(config)) {
        throw new Error('Configuration must be a non-null object');
    }

    const configObj = config as Record<string, unknown>;

    for (const [fieldName, fieldSchema] of Object.entries(schema)) {
        const value = configObj[fieldName];
        const exists = fieldName in configObj;

        if (fieldSchema.required && !exists) {
            throw new Error(`Missing required field: ${fieldName}`);
        }

        if (!exists && fieldSchema.default !== undefined) {
            configObj[fieldName] = fieldSchema.default;
            continue;
        }

        if (!exists && !fieldSchema.required) {
            continue;
        }

        if (exists) {
            const actualType = Array.isArray(value) ? 'array' : typeof value;

            if (fieldSchema.type === 'object') {
                if (!value || typeof value !== 'object' || Array.isArray(value)) {
                    throw new Error(
                        fieldSchema.errorMessage ||
                        `Field '${fieldName}' must be an object`
                    );
                }
            } else if (fieldSchema.type === 'array') {
                if (!Array.isArray(value)) {
                    throw new Error(
                        fieldSchema.errorMessage ||
                        `Field '${fieldName}' must be an array`
                    );
                }
            } else if (actualType !== fieldSchema.type) {
                throw new Error(
                    fieldSchema.errorMessage ||
                    `Field '${fieldName}' must be of type ${fieldSchema.type}, got ${actualType}`
                );
            }

            if (fieldSchema.validate && !fieldSchema.validate(value)) {
                throw new Error(
                    fieldSchema.errorMessage ||
                    `Field '${fieldName}' failed custom validation`
                );
            }
        }
    }
}

export function createSchemaValidator<T>(schema: ConfigSchema): (config: unknown) => asserts config is T {
    return (config: unknown): asserts config is T => {
        validateSchema(config, schema);
    };
}

export const CommonSchemas = {
    url: {
        type: 'string' as FieldType,
        required: true,
        validate: (value: unknown) => {
            if (typeof value !== 'string') return false;
            try {
                new URL(value);
                return true;
            } catch {
                return false;
            }
        },
        errorMessage: 'Must be a valid URL',
    },

    positiveNumber: {
        type: 'number' as FieldType,
        validate: (value: unknown) => typeof value === 'number' && value > 0,
        errorMessage: 'Must be a positive number',
    },

    nonEmptyString: {
        type: 'string' as FieldType,
        required: true,
        validate: (value: unknown) => typeof value === 'string' && value.trim().length > 0,
        errorMessage: 'Must be a non-empty string',
    },

    email: {
        type: 'string' as FieldType,
        validate: (value: unknown) => {
            if (typeof value !== 'string') return false;
            return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
        },
        errorMessage: 'Must be a valid email address',
    },

    port: {
        type: 'number' as FieldType,
        validate: (value: unknown) => {
            return typeof value === 'number' && value >= 1 && value <= 65535;
        },
        errorMessage: 'Must be a valid port number (1-65535)',
    },
};
