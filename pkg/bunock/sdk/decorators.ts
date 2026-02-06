
// Decorators and utilities for enhanced block functionality

/**
 * Timeout configuration for async operations
 */
export interface TimeoutConfig {
    ms: number;
    errorMessage?: string;
}

/**
 * Retry configuration for failed operations
 */
export interface RetryConfig {
    attempts: number;
    backoff?: 'linear' | 'exponential';
    initialDelay?: number;
    maxDelay?: number;
}

/**
 * Wraps a promise with a timeout
 * Rejects if the promise doesn't resolve within the specified time
 */
export function withTimeout<T>(promise: Promise<T>, config: TimeoutConfig): Promise<T> {
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
            reject(new Error(config.errorMessage || `Operation timed out after ${config.ms}ms`));
        }, config.ms);

        promise
            .then((value) => {
                clearTimeout(timer);
                resolve(value);
            })
            .catch((error) => {
                clearTimeout(timer);
                reject(error);
            });
    });
}

/**
 * Retries a function with configurable backoff strategy
 */
export async function withRetry<T>(
    fn: () => Promise<T>,
    config: RetryConfig
): Promise<T> {
    const { attempts, backoff = 'exponential', initialDelay = 100, maxDelay = 5000 } = config;

    let lastError: Error | undefined;

    for (let attempt = 1; attempt <= attempts; attempt++) {
        try {
            return await fn();
        } catch (error) {
            lastError = error instanceof Error ? error : new Error(String(error));

            // Don't retry on last attempt
            if (attempt === attempts) {
                break;
            }

            // Calculate delay based on backoff strategy
            let delay: number;
            if (backoff === 'linear') {
                delay = initialDelay * attempt;
            } else {

                delay = initialDelay * Math.pow(2, attempt - 1);
            }

            // Cap at maxDelay
            delay = Math.min(delay, maxDelay);

            // Wait before retrying
            await new Promise(resolve => setTimeout(resolve, delay));
        }
    }

    throw lastError || new Error('All retry attempts failed');
}

/**
 * Helper to create a timeout wrapper for execute methods
 * Usage: return await executeWithTimeout(async () => { ... }, { ms: 5000 });
 */
export async function executeWithTimeout<T>(
    fn: () => Promise<T>,
    timeoutMs: number,
    errorMessage?: string
): Promise<T> {
    return withTimeout(fn(), { ms: timeoutMs, errorMessage });
}

/**
 * Helper to create a retry wrapper for execute methods
 * Usage: return await executeWithRetry(async () => { ... }, { attempts: 3 });
 */
export async function executeWithRetry<T>(
    fn: () => Promise<T>,
    config: RetryConfig
): Promise<T> {
    return withRetry(fn, config);
}

/**
 * Combined timeout + retry helper
 * Applies timeout to each retry attempt
 */
export async function executeWithTimeoutAndRetry<T>(
    fn: () => Promise<T>,
    timeoutMs: number,
    retryConfig: RetryConfig
): Promise<T> {
    return withRetry(
        () => withTimeout(fn(), { ms: timeoutMs }),
        retryConfig
    );
}
