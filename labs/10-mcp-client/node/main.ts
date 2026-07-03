import { spawn, ChildProcessWithoutNullStreams } from "child_process";
import path from "path";

function defaultServerScript(): string {
  return path.resolve(__dirname, "..", "..", "09-mcp-server-stdio", "python", "main.py");
}

class StreamReader {
  private buffer = Buffer.alloc(0);
  private waiter: (() => void) | null = null;
  private ended = false;

  constructor(stream: NodeJS.ReadableStream) {
    stream.on("data", (chunk: Buffer) => {
      this.buffer = Buffer.concat([this.buffer, chunk]);
      if (this.waiter) {
        this.waiter();
        this.waiter = null;
      }
    });
    stream.on("end", () => {
      this.ended = true;
      if (this.waiter) {
        this.waiter();
        this.waiter = null;
      }
    });
  }

  private async wait(): Promise<void> {
    if (this.buffer.length > 0 || this.ended) return;
    await new Promise<void>((resolve) => {
      this.waiter = resolve;
    });
  }

  async readUntil(sep: string): Promise<Buffer | null> {
    const sepBuf = Buffer.from(sep);
    while (true) {
      const idx = this.buffer.indexOf(sepBuf);
      if (idx !== -1) {
        const result = this.buffer.slice(0, idx);
        this.buffer = this.buffer.slice(idx + sepBuf.length);
        return result;
      }
      if (this.ended) return this.buffer.length > 0 ? this.buffer : null;
      await this.wait();
    }
  }

  async readExactly(n: number): Promise<Buffer | null> {
    while (this.buffer.length < n) {
      if (this.ended) return null;
      await this.wait();
    }
    const result = this.buffer.slice(0, n);
    this.buffer = this.buffer.slice(n);
    return result;
  }
}

class MCPStdioClient {
  private process: ChildProcessWithoutNullStreams | null = null;
  private requestId = 0;
  private reader: StreamReader | null = null;

  constructor(private serverScript?: string) {}

  private nextId(): number {
    return ++this.requestId;
  }

  connect(): void {
    const script =
      this.serverScript || process.env.MCP_SERVER_SCRIPT || defaultServerScript();
    console.log("Starting MCP server:", script);
    this.process = spawn("python", [script], {
      stdio: ["pipe", "pipe", "pipe"],
    });
    this.reader = new StreamReader(this.process.stdout);
  }

  disconnect(): void {
    if (this.process && !this.process.killed) {
      this.process.stdin.end();
      this.process.kill();
    }
    this.process = null;
  }

  private send(message: any): void {
    const body = Buffer.from(JSON.stringify(message), "utf8");
    const data = Buffer.concat([
      Buffer.from(`Content-Length: ${body.length}\r\n\r\n`),
      body,
    ]);
    this.process!.stdin.write(data);
  }

  private async recv(): Promise<any> {
    const headerBytes = await this.reader!.readUntil("\r\n\r\n");
    if (!headerBytes) throw new Error("Server closed stdout");
    const headers: Record<string, string> = {};
    for (const line of headerBytes.toString("utf8").split("\r\n")) {
      const colon = line.indexOf(":");
      if (colon !== -1) {
        headers[line.slice(0, colon).trim().toLowerCase()] = line
          .slice(colon + 1)
          .trim();
      }
    }
    const length = parseInt(headers["content-length"] || "0", 10);
    if (length === 0) throw new Error("Empty message body");
    const body = await this.reader!.readExactly(length);
    if (!body) throw new Error("Server closed stdout while reading body");
    return JSON.parse(body.toString("utf8"));
  }

  async initialize(): Promise<any> {
    this.send({
      jsonrpc: "2.0",
      id: this.nextId(),
      method: "initialize",
      params: { protocolVersion: "2024-11-05", capabilities: {} },
    });
    const result = await this.recv();
    this.send({
      jsonrpc: "2.0",
      id: null,
      method: "notifications/initialized",
    });
    return result;
  }

  async listTools(): Promise<any[]> {
    this.send({ jsonrpc: "2.0", id: this.nextId(), method: "tools/list" });
    const response = await this.recv();
    if (response.error) throw new Error(JSON.stringify(response.error));
    return response.result?.tools || [];
  }

  async callTool(name: string, args: any): Promise<any> {
    this.send({
      jsonrpc: "2.0",
      id: this.nextId(),
      method: "tools/call",
      params: { name, arguments: args },
    });
    const response = await this.recv();
    if (response.error) throw new Error(JSON.stringify(response.error));
    return response.result || {};
  }
}

function parseSSE(lines: string[]): Array<Record<string, string>> {
  const events: Array<Record<string, string>> = [];
  let event: Record<string, string> = {};
  for (const raw of lines) {
    const line = raw.replace(/\r$/, "");
    if (!line) {
      if (Object.keys(event).length > 0) {
        events.push(event);
        event = {};
      }
      continue;
    }
    if (line.startsWith(":")) continue;
    const colon = line.indexOf(":");
    if (colon !== -1) {
      const key = line.slice(0, colon);
      const value = line.slice(colon + 1).replace(/^\s+/, "");
      event[key] = value;
    }
  }
  if (Object.keys(event).length > 0) events.push(event);
  return events;
}

function demoSSE(): void {
  const rawStream = Buffer.from(
    ":heartbeat\n\n" +
      "event: message\n" +
      'data: {"tool": "calculate", "args": {"expression": "1+1"}}\n\n' +
      "event: status\n" +
      "data: processing\n\n" +
      "event: done\n" +
      "data: finished\n\n"
  );
  console.log("\n[SSE transport concept]");
  for (const event of parseSSE(rawStream.toString("utf8").split(/\r?\n/))) {
    console.log("  SSE event:", event);
  }
}

async function main() {
  const client = new MCPStdioClient();
  try {
    client.connect();
    const initResponse = await client.initialize();
    console.log("[initialize]", JSON.stringify(initResponse.result || {}, null, 2));

    const tools = await client.listTools();
    console.log("\n[tools]");
    for (const tool of tools) {
      console.log(`  - ${tool.name}: ${tool.description || ""}`);
    }

    const result = await client.callTool("calculate", { expression: "(10 + 5) / 3" });
    console.log("\n[tools/call calculate]");
    for (const item of result.content || []) {
      console.log(" ", item.text);
    }

    demoSSE();
  } catch (exc: any) {
    console.error("Client failed:", exc);
    process.exit(1);
  } finally {
    client.disconnect();
  }
}

main();
