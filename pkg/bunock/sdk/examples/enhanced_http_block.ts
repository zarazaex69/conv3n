
import { Block, BlockHelpers, executeWithTimeoutAndRetry, createSchemaValidator, CommonSchemas } from "#sdk";

interface HttpConfig {
    url: string;
    method?: string;
    timeout?: number;
    retryAttempts?: number;
}

interface HttpOutput {
    status: number;
    data: unknown;
}

class EnhancedHttpBlock extends Block<HttpConfig, HttpOutput> {

    private schema = {
        url: CommonSchemas.url,
        method: {
            type: 'string' as const,
            default: 'GET',
            validate: (v: unknown) => ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].includes(v as string),
            errorMessage: 'Method must be GET, POST, PUT, DELETE, or PATCH'
        },
        timeout: {
            type: 'number' as const,
            default: 5000,
            validate: (v: unknown) => typeof v === 'number' && v > 0,
        },
        retryAttempts: {
            type: 'number' as const,
            default: 3,
            validate: (v: unknown) => typeof v === 'number' && v >= 1 && v <= 10,
        }
    };

    validate = createSchemaValidator<HttpConfig>(this.schema);

    async execute(config: HttpConfig): Promise<HttpOutput> {

        const result = await executeWithTimeoutAndRetry(
            async () => {
                const response = await fetch(config.url, {
                    method: config.method || 'GET',
                });

                const text = await response.text();
                return {
                    status: response.status,
                    data: BlockHelpers.parseJSON(text),
                };
            },
            config.timeout || 5000,
            { attempts: config.retryAttempts || 3, backoff: 'exponential' }
        );

        return result;
    }

    protected getOutputPort(result: HttpOutput): string {
        return BlockHelpers.getHttpPort(result.status);
    }
}

if (import.meta.main) {
    new EnhancedHttpBlock().run();
}
