import fs from "fs";
import path from "path";
import { createHash } from "crypto";
import { OpenAIClient } from "../../../shared/config/openai_client";

const CHECKPOINT_DIR = path.join(__dirname, ".checkpoints");
fs.mkdirSync(CHECKPOINT_DIR, { recursive: true });

class CheckpointStore {
  path: string;

  constructor(key: string) {
    this.path = path.join(CHECKPOINT_DIR, `${key}.json`);
  }

  load(): any {
    if (fs.existsSync(this.path)) {
      return JSON.parse(fs.readFileSync(this.path, "utf8"));
    }
    return {};
  }

  save(state: any): void {
    const tmp = `${this.path}.tmp`;
    fs.writeFileSync(tmp, JSON.stringify(state, null, 2), "utf8");
    fs.renameSync(tmp, this.path);
  }
}

function exponentialBackoff(attempt: number, baseDelay = 1.0, maxDelay = 30.0): number {
  const delay = Math.min(baseDelay * Math.pow(2, attempt - 1), maxDelay);
  const jitter = delay * 0.1 * (attempt % 2 === 0 ? 1 : -1);
  return delay + jitter;
}

function runWithTimeout<T>(func: () => T | Promise<T>, timeout: number): Promise<T> {
  return Promise.race([
    Promise.resolve(func()),
    new Promise<T>((_, reject) =>
      setTimeout(
        () => reject(new Error(`Step timed out after ${timeout}s`)),
        timeout * 1000
      )
    ),
  ]);
}

class LongTask {
  idempotencyKey: string;
  store: CheckpointStore;
  state: any;
  private flakyAttempts = 0;

  constructor(private inputData: string, private client: OpenAIClient | null) {
    this.idempotencyKey = this.makeKey(inputData);
    this.store = new CheckpointStore(this.idempotencyKey);
    this.state = this.store.load();
  }

  private makeKey(inputData: string): string {
    const key = createHash("sha1")
      .update("6ba7b810-9dad-11d1-80b4-00c04fd430c8" + inputData)
      .digest("hex");
    return `task-${key}`;
  }

  private isComplete(): boolean {
    return (
      this.state.status === "completed" &&
      this.state.idempotency_key === this.idempotencyKey
    );
  }

  private async runStep(
    name: string,
    func: () => any,
    timeout = 5.0,
    maxRetries = 3
  ): Promise<any> {
    if (this.state.completed_steps?.[name]) {
      console.log(`Step '${name}' already completed; skipping`);
      return this.state.results[name];
    }

    console.log(`Executing step '${name}'`);
    let lastError: Error | null = null;
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        const result = await runWithTimeout(func, timeout);
        if (!this.state.completed_steps) this.state.completed_steps = {};
        this.state.completed_steps[name] = true;
        if (!this.state.results) this.state.results = {};
        this.state.results[name] = result;
        this.state.last_step = name;
        this.store.save(this.state);
        console.log(`Step '${name}' succeeded`);
        return result;
      } catch (exc: any) {
        lastError = exc;
        console.warn(`Step '${name}' attempt ${attempt} failed: ${exc.message}`);
        if (attempt < maxRetries) {
          const delay = exponentialBackoff(attempt);
          console.log(`Retrying step '${name}' in ${delay.toFixed(2)}s`);
          await new Promise((r) => setTimeout(r, delay * 1000));
        }
      }
    }
    console.error(`Step '${name}' exhausted retries`);
    throw lastError;
  }

  private async stepFetchData(): Promise<string> {
    console.log(`Fetching data for: ${this.inputData}`);
    if (this.client) {
      const resp = await this.client.chatCompletion({
        messages: [
          { role: "user", content: `Summarize '${this.inputData}' in one sentence.` },
        ],
        max_tokens: 50,
      });
      return resp.choices[0].message.content.trim();
    }
    return `Mock summary for '${this.inputData}'.`;
  }

  private stepProcessData(fetched: string): string {
    this.flakyAttempts += 1;
    if (this.flakyAttempts < 3) {
      throw new Error(`Processing service busy (attempt ${this.flakyAttempts})`);
    }
    return `processed(${fetched})`;
  }

  private stepNotify(processed: string): string {
    return `notification_sent(${processed})`;
  }

  async run(): Promise<{ status: string; results: any }> {
    if (this.isComplete()) {
      console.log(`Task already completed for key ${this.idempotencyKey}; returning cached result`);
      return { status: "completed", results: this.state.results };
    }

    this.state.idempotency_key = this.idempotencyKey;
    this.state.status = this.state.status || "running";
    this.state.completed_steps = this.state.completed_steps || {};
    this.state.results = this.state.results || {};

    const fetched = await this.runStep("fetch", () => this.stepFetchData());
    const processed = await this.runStep("process", () => this.stepProcessData(fetched));
    const notified = await this.runStep("notify", () => this.stepNotify(processed));

    this.state.status = "completed";
    this.store.save(this.state);
    console.log("Task completed successfully");
    return { status: "completed", results: this.state.results };
  }
}

async function main() {
  let client: OpenAIClient | null = null;
  try {
    client = new OpenAIClient();
  } catch (exc: any) {
    console.warn("LLM client disabled:", exc.message);
  }

  const inputData = "reliable agent engineering";
  const task = new LongTask(inputData, client);

  console.log("Starting long-running task...");
  console.log("Idempotency key:", task.idempotencyKey);
  const result = await task.run();
  console.log("\nFinal result:");
  console.log(JSON.stringify(result, null, 2));

  console.log("\nRe-running with the same idempotency key...");
  const task2 = new LongTask(inputData, client);
  const result2 = await task2.run();
  console.log(JSON.stringify(result2, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
