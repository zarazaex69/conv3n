import { Block, BlockHelpers } from "#sdk";

interface LoopConfig {
    items: unknown[];
    mapExpression?: string;
    filterExpression?: string;
}

interface LoopOutput {
    results: unknown[];
    count: number;
    originalCount: number;
}

export class LoopBlock extends Block<LoopConfig, LoopOutput> {
    validate(config: unknown): asserts config is LoopConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertField(config, "items", "array");

        if ("mapExpression" in config && config.mapExpression !== undefined) {
            BlockHelpers.assertField(config, "mapExpression", "string");
            this.validateExpression(config.mapExpression as string);
        }

        if ("filterExpression" in config && config.filterExpression !== undefined) {
            BlockHelpers.assertField(config, "filterExpression", "string");
            this.validateExpression(config.filterExpression as string);
        }
    }

    async execute(config: LoopConfig): Promise<LoopOutput> {
        let items = [...config.items];
        const originalCount = items.length;

        if (config.filterExpression) {
            items = this.applyFilter(items, config.filterExpression);
        }

        if (config.mapExpression) {
            items = this.applyMap(items, config.mapExpression);
        }

        return {
            results: items,
            count: items.length,
            originalCount,
        };
    }

    protected getOutputPort(result: LoopOutput): string {
        return result.count > 0 ? "default" : "empty";
    }

    private validateExpression(expression: string): void {
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

        if (expression.length > 1000) {
            throw new Error("Expression too long (max 1000 characters)");
        }

        try {
            new Function("item", `'use strict'; return (${expression});`);
        } catch (error) {
            throw new Error(`Invalid expression syntax: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private applyMap(items: unknown[], expression: string): unknown[] {
        try {
            const isArrowFunction = expression.includes("=>");

            if (isArrowFunction) {
                const mapFn = new Function("item", "index", "array", `
                    'use strict';
                    const fn = ${expression};
                    return fn(item, index, array);
                `);
                return items.map((item, index, array) => mapFn(item, index, array));
            } else {
                const mapFn = new Function("item", "index", "array", `
                    'use strict';
                    return (${expression});
                `);
                return items.map((item, index, array) => mapFn(item, index, array));
            }
        } catch (error) {
            throw new Error(`Map expression failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }

    private applyFilter(items: unknown[], expression: string): unknown[] {
        try {
            const isArrowFunction = expression.includes("=>");

            if (isArrowFunction) {
                const filterFn = new Function("item", "index", "array", `
                    'use strict';
                    const fn = ${expression};
                    return Boolean(fn(item, index, array));
                `);
                return items.filter((item, index, array) => filterFn(item, index, array));
            } else {
                const filterFn = new Function("item", "index", "array", `
                    'use strict';
                    return Boolean(${expression});
                `);
                return items.filter((item, index, array) => filterFn(item, index, array));
            }
        } catch (error) {
            throw new Error(`Filter expression failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }
}

if (import.meta.main) {
    new LoopBlock().run();
}
