import { serve } from "bun";
import { readFileSync } from "fs";

const socketPath = process.argv[2];

if (!socketPath) {
    console.error("Usage: bun run worker_server.ts <socket_path>");
    process.exit(1);
}

interface TaskRequest {
    task_id: string;
    script_path: string;
    input: any;
}

interface TaskResponse {
    task_id: string;
    data?: any;
    error?: string;
}

async function executeScript(scriptPath: string, input: any): Promise<any> {
    const module = await import(scriptPath);
    
    if (module.default && typeof module.default.execute === "function") {
        const instance = new module.default();
        return await instance.execute(input.config, input.input);
    }
    
    if (typeof module.execute === "function") {
        return await module.execute(input);
    }
    
    throw new Error(`Script ${scriptPath} does not export a valid block`);
}

const server = serve({
    unix: socketPath,
    
    async fetch(req) {
        try {
            const task: TaskRequest = await req.json();
            
            const result = await executeScript(task.script_path, task.input);
            
            const response: TaskResponse = {
                task_id: task.task_id,
                data: result,
            };
            
            return new Response(JSON.stringify(response), {
                headers: { "Content-Type": "application/json" },
            });
            
        } catch (error) {
            const response: TaskResponse = {
                task_id: "unknown",
                error: error instanceof Error ? error.message : String(error),
            };
            
            return new Response(JSON.stringify(response), {
                status: 500,
                headers: { "Content-Type": "application/json" },
            });
        }
    },
});

console.log(`Worker listening on ${socketPath}`);

process.on("SIGTERM", () => {
    server.stop();
    process.exit(0);
});
