import { Block, BlockHelpers } from "#sdk";

interface SetVarConfig {
    name: string;
    value: unknown;
}

interface SetVarOutput {
    action: string;
    name: string;
    value: unknown;
}

export class SetVarBlock extends Block<SetVarConfig, SetVarOutput> {
    validate(config: unknown): asserts config is SetVarConfig {
        BlockHelpers.assertObject(config);
        BlockHelpers.assertNonEmptyString(config, "name");

        if (!("value" in config)) {
            throw new Error("Missing required field: value");
        }
    }

    async execute(config: SetVarConfig): Promise<SetVarOutput> {
        return {
            action: "set_var",
            name: config.name,
            value: config.value,
        };
    }
}

if (import.meta.main) {
    new SetVarBlock().run();
}
