import { BlockHelpers } from "./sdk.ts";

export { BlockHelpers };

export interface TriggerConfig extends Record<string, unknown> {}

export interface TriggerContext<TConfig extends TriggerConfig = TriggerConfig> {
    config: TConfig;
    fire: <TPayload = unknown>(payload: TPayload) => Promise<unknown>;
}

export interface TriggerMessage {
    type: string;
    config?: unknown;
    payload?: unknown;
    requestId?: string;
}

export abstract class Trigger<TConfig extends TriggerConfig = TriggerConfig> {
    protected context: TriggerContext<TConfig> | null = null;
    private messageHandlers: Map<string, (msg: TriggerMessage) => Promise<void>> = new Map();

    abstract validate(config: unknown): asserts config is TConfig;

    abstract start(ctx: TriggerContext<TConfig>): Promise<void>;

    abstract stop(ctx: TriggerContext<TConfig>): Promise<void>;

    protected async handleMessage(message: TriggerMessage, ctx: TriggerContext<TConfig>): Promise<void> {
        const handler = this.messageHandlers.get(message.type);
        if (handler) {
            await handler(message);
        }
    }

    protected registerMessageHandler(type: string, handler: (msg: TriggerMessage) => Promise<void>): void {
        this.messageHandlers.set(type, handler);
    }

    async run(): Promise<void> {
        try {
            await this.startMessageLoop();
        } catch (error) {
            await this.writeError(error);
            process.exit(1);
        }
    }

    private async startMessageLoop(): Promise<void> {
        const reader = Bun.stdin.stream().getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
            const { done, value } = await reader.read();
            
            if (done) {
                break;
            }

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split("\n");
            buffer = lines.pop() || "";

            for (const line of lines) {
                if (!line.trim()) continue;

                try {
                    const message = JSON.parse(line) as TriggerMessage;
                    await this.processMessage(message);
                } catch (error) {
                    console.error("Failed to parse message:", error);
                }
            }
        }
    }

    private async processMessage(message: TriggerMessage): Promise<void> {
        if (message.type === "start") {
            this.validate(message.config);
            
            this.context = {
                config: message.config as TConfig,
                fire: async <TPayload>(payload: TPayload) => {
                    return await this.emitEvent(payload);
                }
            };

            await this.start(this.context);
            await this.writeReady();
        } else if (message.type === "kill") {
            if (this.context) {
                await this.stop(this.context);
            }
            process.exit(0);
        } else {
            if (this.context) {
                await this.handleMessage(message, this.context);
            }
        }
    }

    private async emitEvent<TPayload>(payload: TPayload): Promise<unknown> {
        const requestId = this.generateRequestId();
        
        await this.writeMessage({
            type: "event",
            payload,
            requestId
        });

        return await this.waitForResponse(requestId);
    }

    private async waitForResponse(requestId: string): Promise<unknown> {
        return new Promise((resolve, reject) => {
            const timeout = setTimeout(() => {
                reject(new Error(`Timeout waiting for response to ${requestId}`));
            }, 30000);

            const checkResponse = async () => {
                const reader = Bun.stdin.stream().getReader();
                const decoder = new TextDecoder();
                
                try {
                    const { value } = await reader.read();
                    if (value) {
                        const message = JSON.parse(decoder.decode(value)) as TriggerMessage;
                        
                        if (message.requestId === requestId) {
                            clearTimeout(timeout);
                            resolve(message.payload);
                        } else {
                            await checkResponse();
                        }
                    }
                } catch (error) {
                    clearTimeout(timeout);
                    reject(error);
                }
            };

            checkResponse();
        });
    }

    private generateRequestId(): string {
        return `req_${Date.now()}_${Math.random().toString(36).substring(2, 9)}`;
    }

    private async writeMessage(message: TriggerMessage): Promise<void> {
        await Bun.write(Bun.stdout, JSON.stringify(message) + "\n");
    }

    private async writeReady(): Promise<void> {
        await this.writeMessage({ type: "ready" });
    }

    private async writeError(error: unknown): Promise<void> {
        const errorMessage = error instanceof Error ? error.message : String(error);
        const errorStack = error instanceof Error ? error.stack : undefined;
        
        await this.writeMessage({
            type: "error",
            payload: {
                message: errorMessage,
                stack: errorStack
            }
        });
    }
}
