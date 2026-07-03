import { PassThrough, Readable } from "stream";

const DOCUMENTS: Record<string, string> = {};

function updateDocument(uri: string, text: string): void {
  DOCUMENTS[uri] = text;
}

function findDefinition(uri: string, position: any): any | null {
  const text = DOCUMENTS[uri] || "";
  const lines = text.split("\n");
  const lineIdx = position.line || 0;
  const charIdx = position.character || 0;
  if (lineIdx < 0 || lineIdx >= lines.length) return null;
  const line = lines[lineIdx];
  const match = (line.slice(charIdx) || "").match(/[A-Za-z0-9_]+/);
  if (!match) return null;
  const word = match[0];

  const pattern = new RegExp(`^def\\s+${word.replace(/[.*+?^${}()|[\]\\]/g, "\\$\u0026")}\\s*\\(`, "m");
  for (const docUri of Object.keys(DOCUMENTS)) {
    const docText = DOCUMENTS[docUri];
    const m = docText.match(pattern);
    if (m) {
      const startLine = docText.slice(0, m.index).split("\n").length - 1;
      return {
        uri: docUri,
        range: {
          start: { line: startLine, character: 0 },
          end: { line: startLine, character: `def ${word}(`.length },
        },
      };
    }
  }
  return null;
}

// ---------- JSON-RPC framing ----------

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

async function* readMessages(reader: StreamReader): AsyncGenerator<any> {
  while (true) {
    const headerBytes = await reader.readUntil("\r\n\r\n");
    if (!headerBytes) break;
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
    if (length === 0) break;
    const body = await reader.readExactly(length);
    if (!body) break;
    yield JSON.parse(body.toString("utf8"));
  }
}

function writeMessage(stream: NodeJS.WritableStream, message: any): void {
  const body = Buffer.from(JSON.stringify(message), "utf8");
  stream.write(`Content-Length: ${body.length}\r\n\r\n`);
  stream.write(body);
  (stream as any).flush?.();
}

// ---------- request handlers ----------

function makeResponse(requestId: any, result: any): any {
  return { jsonrpc: "2.0", id: requestId, result };
}

function makeError(requestId: any, code: number, message: string): any {
  return { jsonrpc: "2.0", id: requestId, error: { code, message } };
}

function handleRequest(request: any): any | null {
  const method = request.method;
  const params = request.params || {};
  const reqId = request.id;

  console.error("Received", method);

  if (method === "initialize") {
    return makeResponse(reqId, {
      capabilities: {
        textDocumentSync: { openClose: true, change: 0 },
        definitionProvider: true,
      },
      serverInfo: { name: "agent-labs-lsp", version: "0.1.0" },
    });
  }

  if (method === "initialized") return null;

  if (method === "textDocument/didOpen") {
    const doc = params.textDocument || {};
    updateDocument(doc.uri || "", doc.text || "");
    return null;
  }

  if (method === "textDocument/definition") {
    const td = params.textDocument || {};
    const pos = params.position || {};
    const location = findDefinition(td.uri || "", pos);
    return makeResponse(reqId, location);
  }

  if (method === "shutdown") return makeResponse(reqId, null);

  if (method === "exit") return null;

  return makeError(reqId, -32601, `Method not found: ${method}`);
}

async function serve(
  stdin: NodeJS.ReadableStream = process.stdin,
  stdout: NodeJS.WritableStream = process.stdout
): Promise<void> {
  const reader = new StreamReader(stdin);
  console.error("LSP server ready on stdio");
  for await (const request of readMessages(reader)) {
    const method = request.method;
    const response = handleRequest(request);
    if (response !== null) {
      writeMessage(stdout, response);
    }
    if (method === "exit") break;
  }
}

// ---------- smoke test ----------

async function smokeTest(): Promise<void> {
  const sampleCode = [
    'def greet(name):',
    '    return f"Hello, {name}!"',
    '',
    'print(greet("world"))',
  ].join("\n");
  const uri = "file:///tmp/sample.py";

  const requests = [
    {
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: { processId: null, rootUri: null, capabilities: {} },
    },
    { jsonrpc: "2.0", method: "initialized", params: {} },
    {
      jsonrpc: "2.0",
      method: "textDocument/didOpen",
      params: {
        textDocument: { uri, languageId: "python", text: sampleCode },
      },
    },
    {
      jsonrpc: "2.0",
      id: 2,
      method: "textDocument/definition",
      params: {
        textDocument: { uri },
        position: { line: 3, character: 6 },
      },
    },
    { jsonrpc: "2.0", id: 3, method: "shutdown" },
    { jsonrpc: "2.0", method: "exit" },
  ];

  const chunks: Buffer[] = [];
  for (const req of requests) {
    const body = Buffer.from(JSON.stringify(req), "utf8");
    chunks.push(Buffer.from(`Content-Length: ${body.length}\r\n\r\n`));
    chunks.push(body);
  }
  const stdin = Readable.from(chunks);
  const stdout = new PassThrough();
  const output: Buffer[] = [];
  stdout.on("data", (c) => output.push(c));

  await serve(stdin, stdout);
  console.log("Smoke test output:");
  console.log(Buffer.concat(output).toString("utf8"));
}

async function main() {
  if (process.argv.includes("--smoke")) {
    await smokeTest();
  } else {
    await serve();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
