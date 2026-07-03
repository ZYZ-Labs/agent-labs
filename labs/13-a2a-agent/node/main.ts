import http from "http";
import { randomUUID } from "crypto";

const AGENT_CARD = {
  name: "agent-labs-echo-agent",
  description: "A minimal A2A agent that echoes input after a short delay.",
  url: "http://localhost:8123",
  version: "0.1.0",
  capabilities: {
    streaming: false,
    pushNotifications: false,
  },
  skills: [
    {
      id: "echo",
      name: "Echo",
      description: "Returns the input text as the task result.",
    },
  ],
};

const TASKS: Record<string, any> = {};
const TASK_LOCK: Promise<void> = Promise.resolve();

async function withLock<T>(fn: () => T): Promise<T> {
  return await TASK_LOCK.then(fn);
}

function createTask(message: any): any {
  const taskId = randomUUID();
  const task: any = {
    id: taskId,
    status: { state: "submitted", timestamp: Date.now() },
    messages: [message],
    artifacts: [],
  };
  TASKS[taskId] = task;

  setTimeout(() => {
    task.status = { state: "working", timestamp: Date.now() };
  }, 2000);
  setTimeout(() => {
    const text = message.parts?.[0]?.text || "";
    task.status = {
      state: "completed",
      timestamp: Date.now(),
      message: {
        role: "agent",
        parts: [{ type: "text", text: `Echo: ${text}` }],
      },
    };
  }, 4000);

  return task;
}

function sendJson(res: http.ServerResponse, status: number, payload: any): void {
  const body = JSON.stringify(payload);
  res.writeHead(status, { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body) });
  res.end(body);
}

function startServer(host = "localhost", port = 8123): http.Server {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url || "/", `http://${req.headers.host}`);
    if (req.method === "GET" && url.pathname === "/.well-known/agent.json") {
      sendJson(res, 200, AGENT_CARD);
      return;
    }
    if (req.method === "GET" && url.pathname.startsWith("/tasks/")) {
      const taskId = url.pathname.split("/").pop();
      const task = taskId ? TASKS[taskId] : null;
      if (task) sendJson(res, 200, task);
      else sendJson(res, 404, { error: "Task not found" });
      return;
    }
    if (req.method === "POST" && url.pathname === "/tasks/send") {
      let body = "";
      req.on("data", (chunk) => (body += chunk));
      req.on("end", () => {
        try {
          const payload = body ? JSON.parse(body) : {};
          const task = createTask(payload.message || {});
          sendJson(res, 200, task);
        } catch {
          sendJson(res, 400, { error: "Invalid JSON" });
        }
      });
      return;
    }
    sendJson(res, 404, { error: "Not found" });
  });
  server.listen(port, host, () => {
    console.log(`Agent server listening on http://${host}:${port}`);
  });
  return server;
}

function fetchJson(url: string, options: http.RequestOptions & { body?: string } = {}): Promise<any> {
  return new Promise((resolve, reject) => {
    const req = http.request(url, options, (res) => {
      let data = "";
      res.on("data", (chunk) => (data += chunk));
      res.on("end", () => {
        try {
          resolve(JSON.parse(data));
        } catch {
          resolve(data);
        }
      });
    });
    req.on("error", reject);
    if (options.body) req.write(options.body);
    req.end();
  });
}

async function fetchAgentCard(baseUrl: string): Promise<any> {
  return fetchJson(`${baseUrl}/.well-known/agent.json`);
}

async function submitTask(baseUrl: string, text: string): Promise<any> {
  const payload = {
    message: {
      role: "user",
      parts: [{ type: "text", text }],
    },
  };
  return fetchJson(`${baseUrl}/tasks/send`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

async function getTask(baseUrl: string, taskId: string): Promise<any> {
  return fetchJson(`${baseUrl}/tasks/${taskId}`);
}

async function pollTask(
  baseUrl: string,
  taskId: string,
  timeout = 30000,
  interval = 500
): Promise<any> {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const task = await getTask(baseUrl, taskId);
    const state = task.status?.state;
    console.log(`Task ${taskId.slice(0, 8)} state: ${state}`);
    if (state === "completed" || state === "failed") return task;
    await new Promise((r) => setTimeout(r, interval));
  }
  throw new Error(`Task ${taskId} did not complete within ${timeout}ms`);
}

async function main() {
  const baseUrl = (process.env.A2A_AGENT_URL || "http://localhost:8123").replace(/\/$/, "");
  let ownServer: http.Server | null = null;

  try {
    try {
      await fetchAgentCard(baseUrl);
      console.log(`Using existing agent at ${baseUrl}`);
    } catch {
      console.log(`No agent found at ${baseUrl}; starting local agent`);
      ownServer = startServer();
      for (let i = 0; i < 20; i++) {
        try {
          await fetchAgentCard(baseUrl);
          break;
        } catch {
          await new Promise((r) => setTimeout(r, 100));
        }
      }
    }

    const card = await fetchAgentCard(baseUrl);
    console.log("\n[Agent Card]");
    console.log(JSON.stringify(card, null, 2));

    const task = await submitTask(baseUrl, "Hello from the A2A client!");
    console.log("\n[Submitted Task]");
    console.log(JSON.stringify(task, null, 2));

    const final = await pollTask(baseUrl, task.id);
    console.log("\n[Final Task]");
    console.log(JSON.stringify(final, null, 2));
  } finally {
    if (ownServer) ownServer.close();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
