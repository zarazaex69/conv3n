import { Trigger, type TriggerContext, BlockHelpers } from "#sdk";

interface CommandWatchConfig extends Record<string, unknown> {
    command: string;
    args?: string[];
    interval: number;
    expectedOutput?: string;
    matchPattern?: string;
}

interface CommandResult {
    stdout: string;
    stderr: string;
    exitCode: number;
    timestamp: number;
    matched: boolean;
}

class CommandWatchTrigger extends Trigger<CommandWatchConfig> {
    private intervalId: Timer | null = null;
    private lastOutput: string = "";

    validate(config: unknown): asserts config is CommandWatchConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "command");
        BlockHelpers.assertField(config, "interval", "number");

        const interval = config.interval as number;
        if (interval < 100) {
            throw new Error("Interval must be at least 100ms");
        }
    }

    async start(ctx: TriggerContext<CommandWatchConfig>): Promise<void> {
        const { command, args = [], interval, expectedOutput, matchPattern } = ctx.config;

        const checkCommand = async () => {
            try {
                const proc = Bun.spawn([command, ...args], {
                    stdout: "pipe",
                    stderr: "pipe"
                });

                const stdout = await new Response(proc.stdout).text();
                const stderr = await new Response(proc.stderr).text();
                const exitCode = await proc.exited;

                let matched = false;

                if (expectedOutput !== undefined) {
                    matched = stdout.includes(expectedOutput);
                } else if (matchPattern !== undefined) {
                    const regex = new RegExp(matchPattern);
                    matched = regex.test(stdout);
                } else {
                    matched = stdout !== this.lastOutput;
                }

                if (matched) {
                    const result: CommandResult = {
                        stdout,
                        stderr,
                        exitCode,
                        timestamp: Date.now(),
                        matched: true
                    };

                    await ctx.fire(result);
                    this.lastOutput = stdout;
                }
            } catch (error) {
                console.error("Command execution failed:", error);
            }
        };

        await checkCommand();

        this.intervalId = setInterval(checkCommand, interval);

        console.log(`Command watch trigger started: ${command} (interval: ${interval}ms)`);
    }

    async stop(ctx: TriggerContext<CommandWatchConfig>): Promise<void> {
        if (this.intervalId) {
            clearInterval(this.intervalId);
            this.intervalId = null;
            console.log(`Command watch trigger stopped: ${ctx.config.command}`);
        }
    }
}

if (import.meta.main) {
    new CommandWatchTrigger().run();
}
