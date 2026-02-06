export type VariableScope = "global" | "workflow" | "execution";

export interface VariableOptions {
    scope?: VariableScope;
    ttlSeconds?: number;
}

export interface VariableCommand {
    action: "set" | "get" | "delete";
    name: string;
    value?: unknown;
    options?: VariableOptions;
}

export interface VariableResult {
    success: boolean;
    value?: unknown;
    error?: string;
}

export class VariableManager {
    private commands: VariableCommand[] = [];

    set(name: string, value: unknown, options?: VariableOptions): void {
        this.commands.push({
            action: "set",
            name,
            value,
            options: options || { scope: "execution" },
        });
    }

    get(name: string): void {
        this.commands.push({
            action: "get",
            name,
        });
    }

    delete(name: string): void {
        this.commands.push({
            action: "delete",
            name,
        });
    }

    setGlobal(name: string, value: unknown, ttlSeconds?: number): void {
        this.set(name, value, { scope: "global", ttlSeconds });
    }

    setWorkflow(name: string, value: unknown, ttlSeconds?: number): void {
        this.set(name, value, { scope: "workflow", ttlSeconds });
    }

    setExecution(name: string, value: unknown, ttlSeconds?: number): void {
        this.set(name, value, { scope: "execution", ttlSeconds });
    }

    getCommands(): VariableCommand[] {
        return this.commands;
    }

    clear(): void {
        this.commands = [];
    }
}
