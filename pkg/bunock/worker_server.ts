import { createServer } from "net";
import { unlinkSync, existsSync } from "fs";

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
    
    let BlockClass = module.default;
    
    if (!BlockClass) {
        const exportedClasses = Object.values(module).filter(
            (exp) => typeof exp === "function" && exp.prototype
        );
        
        if (exportedClasses.length > 0) {
            BlockClass = exportedClasses[0] as any;
        }
    }
    
    if (BlockClass) {
        const instance = new BlockClass();
        
        if (typeof instance.run === "function") {
            let capturedOutput: any = null;
            
            const originalWrite = Bun.write;
            (Bun as any).write = async (dest: any, data: any) => {
                if (dest === Bun.stdout || dest === 1) {
                    try {
                        const str = typeof data === "string" ? data : data.toString();
                        capturedOutput = JSON.parse(str);
                    } catch (e) {
                        capturedOutput = data;
                    }
                    return data.length || 0;
                }
                return originalWrite(dest, data);
            };
            
            const originalStdin = Bun.stdin;
            (Bun as any).stdin = {
                json: async () => input,
            };
            
            try {
                await instance.run();
                return capturedOutput;
            } finally {
                (Bun as any).write = originalWrite;
                (Bun as any).stdin = originalStdin;
            }
        }
        
        if (typeof instance.execute === "function") {
            return await instance.execute(input.config, input.input);
        }
    }
    
    if (typeof module.execute === "function") {
        return await module.execute(input);
    }
    
    throw new Error(`Script ${scriptPath} does not export a valid block`);
}

if (existsSync(socketPath)) {
    unlinkSync(socketPath);
}

const server = createServer((socket) => {
    console.log("New connection established");
    let buffer = "";
    
    socket.on("data", async (chunk) => {
        buffer += chunk.toString();
        
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";
        
        for (const line of lines) {
            if (!line.trim()) continue;
            
            try {
                const task: TaskRequest = JSON.parse(line);
                console.log(`Received task: ${task.task_id}`);
                
                try {
                    const result = await executeScript(task.script_path, task.input);
                    
                    const response: TaskResponse = {
                        task_id: task.task_id,
                        data: result,
                    };
                    
                    socket.write(JSON.stringify(response) + "\n");
                    console.log(`Sent response for task: ${task.task_id}`);
                } catch (error) {
                    console.error(`Task ${task.task_id} failed:`, error);
                    const response: TaskResponse = {
                        task_id: task.task_id,
                        error: error instanceof Error ? error.message : String(error),
                    };
                    
                    socket.write(JSON.stringify(response) + "\n");
                }
            } catch (parseError) {
                console.error("Failed to parse task:", parseError);
            }
        }
    });
    
    socket.on("error", (err) => {
        console.error("Socket error:", err);
    });
    
    socket.on("close", () => {
        console.log("Connection closed");
    });
});

server.listen(socketPath, () => {
    console.log(`Worker listening on ${socketPath}`);
});

process.on("SIGTERM", () => {
    server.close();
    if (existsSync(socketPath)) {
        unlinkSync(socketPath);
    }
    process.exit(0);
});

process.on("SIGINT", () => {
    server.close();
    if (existsSync(socketPath)) {
        unlinkSync(socketPath);
    }
    process.exit(0);
});
