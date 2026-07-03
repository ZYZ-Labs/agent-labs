import { PassThrough, Readable } from "stream";

// ---------- safe calculator ----------

interface Token {
  type: "number" | "op" | "paren";
  value: string;
}

function tokenize(expression: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  const s = expression.replace(/\s+/g, "");
  while (i < s.length) {
    const c = s[i];
    if (/\d/.test(c) || c === ".") {
      let j = i;
      while (j < s.length && (/\d/.test(s[j]) || s[j] === ".")) j++;
      tokens.push({ type: "number", value: s.slice(i, j) });
      i = j;
    } else if ("+-*/%^".includes(c)) {
      tokens.push({ type: "op", value: c });
      i++;
    } else if (c === "(" || c === ")") {
      tokens.push({ type: "paren", value: c });
      i++;
    } else {
      throw new Error(`Invalid character: ${c}`);
    }
  }
  return tokens;
}

class Parser {
  pos = 0;
  constructor(private tokens: Token[]) {}

  peek(): Token | undefined {
    return this.tokens[this.pos];
  }

  consume(): Token | undefined {
    return this.tokens[this.pos++];
  }

  expect(value: string): Token {
    const t = this.consume();
    if (!t || t.value !== value) throw new Error(`Expected ${value}`);
    return t;
  }

  parse(): number {
    const result = this.expr();
    if (this.pos !== this.tokens.length) {
      throw new Error("Unexpected token at end");
    }
    return result;
  }

  expr(): number {
    let left = this.term();
    while (this.peek()?.value === "+" || this.peek()?.value === "-") {
      const op = this.consume()!.value;
      const right = this.term();
      left = op === "+" ? left + right : left - right;
    }
    return left;
  }

  term(): number {
    let left = this.factor();
    while (
      this.peek()?.value === "*" ||
      this.peek()?.value === "/" ||
      this.peek()?.value === "%"
    ) {
      const op = this.consume()!.value;
      const right = this.factor();
      if (op === "*") left = left * right;
      else if (op === "/") left = left / right;
      else left = left % right;
    }
    return left;
  }

  factor(): number {
    if (this.peek()?.value === "+") {
      this.consume();
      return this.factor();
    }
    if (this.peek()?.value === "-") {
      this.consume();
      return -this.factor();
    }
    let left = this.primary();
    if (this.peek()?.value === "^") {
      this.consume();
      const right = this.factor(); // right-associative
      left = Math.pow(left, right);
    }
    return left;
  }

  primary(): number {
    const t = this.peek();
    if (!t) throw new Error("Unexpected end of expression");
    if (t.value === "(") {
      this.consume();
      const val = this.expr();
      this.expect(")");
      return val;
    }
    if (t.type === "number") {
      this.consume();
      return parseFloat(t.value);
    }
    throw new Error(`Unexpected token: ${t.value}`);
  }
}

function calculate(expression: string): any {
  try {
    const value = new Parser(tokenize(expression)).parse();
    return { content: [{ type: "text", text: String(value) }], isError: false };
  } catch (exc: any) {
    return {
      content: [{ type: "text", text: `Invalid expression: ${exc.message}` }],
      isError: true,
    };
  }
}

// ---------- tools ----------

const TOOLS = [
  {
    name: "get_current_time",
    description: "Return the current time in ISO 8601 format.",
    inputSchema: {
      type: "object",
      properties: {
        timezone: {
          type: "string",
          description: "IANA timezone name, e.g. UTC or Asia/Shanghai.",
        },
      },
    },
  },
  {
    name: "calculate",
    description: "Evaluate a simple arithmetic expression.",
    inputSchema: {
      type: "object",
      properties: {
        expression: {
          type: "string",
          description:
            "Arithmetic expression with +, -, *, /, parentheses, numbers.",
        },
      },
      required: ["expression"],
    },
  },
];

function getCurrentTime(timezone = "UTC"): any {
  if (timezone.toUpperCase() !== "UTC") {
    return {
      content: [
        {
          type: "text",
          text: `Unsupported timezone: ${timezone}. Using UTC.`,
        },
      ],
      isError: true,
    };
  }
  return {
    content: [{ type: "text", text: new Date().toISOString() }],
    isError: false,
  };
}

const TOOL_HANDLERS: Record<string, (args: any) => any> = {
  get_current_time: getCurrentTime,
  calculate: calculate,
};

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

async function* readMessages(
  reader: StreamReader
): AsyncGenerator<any> {
  while (true) {
    const headerBytes = await reader.readUntil("\r\n\r\n");
    if (!headerBytes) break;
    const headerText = headerBytes.toString("utf8");
    const headers: Record<string, string> = {};
    for (const line of headerText.split("\r\n")) {
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
    try {
      yield JSON.parse(body.toString("utf8"));
    } catch {
      yield { parseError: true };
    }
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
      protocolVersion: "2024-11-05",
      capabilities: { tools: {} },
      serverInfo: { name: "agent-labs-mcp-server", version: "0.1.0" },
    });
  }

  if (method === "notifications/initialized") return null;

  if (method === "tools/list") {
    return makeResponse(reqId, { tools: TOOLS });
  }

  if (method === "tools/call") {
    const name = params.name;
    const args = params.arguments || {};
    const handler = TOOL_HANDLERS[name];
    if (!handler) {
      return makeError(reqId, -32601, `Unknown tool: ${name}`);
    }
    try {
      return makeResponse(reqId, handler(args));
    } catch (exc: any) {
      return makeError(reqId, -32603, `Tool error: ${exc.message}`);
    }
  }

  return makeError(reqId, -32601, `Method not found: ${method}`);
}

async function serve(
  stdin: NodeJS.ReadableStream = process.stdin,
  stdout: NodeJS.WritableStream = process.stdout
): Promise<void> {
  const reader = new StreamReader(stdin);
  console.error("MCP server ready on stdio");
  for await (const request of readMessages(reader)) {
    if (request.parseError) {
      writeMessage(stdout, makeError(null, -32700, "Parse error"));
      continue;
    }
    const response = handleRequest(request);
    if (response !== null) {
      writeMessage(stdout, response);
    }
  }
  console.error("EOF reached; shutting down");
}

// ---------- smoke test ----------

async function smokeTest(): Promise<void> {
  const requests = [
    {
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: { protocolVersion: "2024-11-05", capabilities: {} },
    },
    { jsonrpc: "2.0", id: null, method: "notifications/initialized" },
    { jsonrpc: "2.0", id: 2, method: "tools/list" },
    {
      jsonrpc: "2.0",
      id: 3,
      method: "tools/call",
      params: { name: "get_current_time", arguments: { timezone: "UTC" } },
    },
    {
      jsonrpc: "2.0",
      id: 4,
      method: "tools/call",
      params: { name: "calculate", arguments: { expression: "(2 + 3) * 4" } },
    },
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
