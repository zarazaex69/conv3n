import { Block, BlockHelpers } from "#sdk";

interface GetVarConfig {
    name: string;
    value?: unknown;
}

interface GetVarOutput {
    action: string;
    name: string;
    value: unknown;
}

export class GetVarBlock extends Block<GetVarConfig, GetVarOutput> {
    validate(config: unknown): asserts config is GetVarConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "name");
    }

    async execute(config: GetVarConfig): Promise<GetVarOutput> {
        return {
            action: "get_var",
            name: config.name,
            value: config.value,
        };
    }
}

if (import.meta.main) {
    new GetVarBlock().run();
}
