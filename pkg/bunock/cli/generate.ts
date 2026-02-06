#!/usr/bin/env bun

import { readdir, readFile, writeFile } from "fs/promises";
import { join, relative, dirname } from "path";
import { existsSync } from "fs";

interface BlockMetadata {
    name: string;
    type: string;
    description: string;
    category: string;
    inputs?: Array<{ name: string; description?: string }>;
    outputs?: Array<{ name: string; description?: string }>;
    configSchema?: Record<string, unknown>;
}

async function extractMetadata(filePath: string): Promise<BlockMetadata | null> {
    const content = await readFile(filePath, "utf-8");

    const nameMatch = content.match(/@block\s+(\S+)/);
    const descMatch = content.match(/@description\s+(.+)/);
    const categoryMatch = content.match(/@category\s+(\S+)/);

    if (!nameMatch) {
        return null;
    }

    const metadata: BlockMetadata = {
        name: nameMatch[1],
        type: nameMatch[1],
        description: descMatch?.[1] || "",
        category: categoryMatch?.[1] || "custom",
    };

    const inputsMatch = content.match(/@inputs\s+(.+)/);
    if (inputsMatch) {
        try {
            metadata.inputs = JSON.parse(inputsMatch[1]);
        } catch {}
    }

    const outputsMatch = content.match(/@outputs\s+(.+)/);
    if (outputsMatch) {
        try {
            metadata.outputs = JSON.parse(outputsMatch[1]);
        } catch {}
    }

    return metadata;
}

async function scanDirectory(dir: string, baseDir: string): Promise<void> {
    const entries = await readdir(dir, { withFileTypes: true });

    for (const entry of entries) {
        const fullPath = join(dir, entry.name);

        if (entry.isDirectory()) {
            await scanDirectory(fullPath, baseDir);
            continue;
        }

        if (!entry.name.endsWith(".ts") || entry.name.endsWith(".test.ts")) {
            continue;
        }

        const metadata = await extractMetadata(fullPath);
        if (!metadata) {
            continue;
        }

        const manifestPath = fullPath.replace(/\.ts$/, ".block.json");
        const scriptPath = relative(dirname(manifestPath), fullPath);

        const manifest = {
            name: metadata.name,
            type: metadata.type,
            description: metadata.description,
            scriptPath: scriptPath,
            category: metadata.category,
            inputs: metadata.inputs || [{ name: "main" }],
            outputs: metadata.outputs || [{ name: "default" }],
            configSchema: metadata.configSchema,
        };

        await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
        console.log(`Generated: ${relative(baseDir, manifestPath)}`);
    }
}

async function main() {
    const args = process.argv.slice(2);
    const targetDir = args[0] || "pkg/blocks";

    if (!existsSync(targetDir)) {
        console.error(`Directory not found: ${targetDir}`);
        process.exit(1);
    }

    console.log(`Scanning ${targetDir} for blocks...`);
    await scanDirectory(targetDir, targetDir);
    console.log("Done!");
}

main().catch((err) => {
    console.error("Error:", err);
    process.exit(1);
});
