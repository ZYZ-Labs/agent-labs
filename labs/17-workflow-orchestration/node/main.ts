import { OpenAIClient, ChatMessage } from "../../../shared/config/openai_client";

class Task {
  result: any = null;
  error: Error | null = null;

  constructor(
    public name: string,
    public func: (...args: any[]) => any,
    public deps: string[] = [],
    public retries = 2
  ) {}
}

class WorkflowEngine {
  tasks: Record<string, Task> = {};

  addTask(task: Task): void {
    this.tasks[task.name] = task;
  }

  private topologicalSort(): string[] {
    const inDegree: Record<string, number> = {};
    const dependents: Record<string, string[]> = {};
    for (const name of Object.keys(this.tasks)) {
      inDegree[name] = 0;
      dependents[name] = [];
    }
    for (const [name, task] of Object.entries(this.tasks)) {
      for (const dep of task.deps) {
        if (!(dep in this.tasks)) {
          throw new Error(`Task ${name} depends on unknown task ${dep}`);
        }
        inDegree[name] += 1;
        dependents[dep].push(name);
      }
    }

    const queue = Object.entries(inDegree)
      .filter(([, deg]) => deg === 0)
      .map(([name]) => name);
    const ordered: string[] = [];
    while (queue.length > 0) {
      const current = queue.shift()!;
      ordered.push(current);
      for (const dependent of dependents[current]) {
        inDegree[dependent] -= 1;
        if (inDegree[dependent] === 0) queue.push(dependent);
      }
    }

    if (ordered.length !== Object.keys(this.tasks).length) {
      throw new Error("Cycle detected in task dependencies");
    }
    return ordered;
  }

  async run(): Promise<Record<string, any>> {
    const order = this.topologicalSort();
    console.log("Execution order:", order.join(", "));
    for (const name of order) {
      const task = this.tasks[name];
      const depsResults: Record<string, any> = {};
      for (const dep of task.deps) {
        depsResults[dep] = this.tasks[dep].result;
      }
      for (let attempt = 1; attempt <= task.retries; attempt++) {
        try {
          console.log(`Running task '${name}' (attempt ${attempt}/${task.retries})`);
          task.result = await Promise.resolve(task.func(depsResults));
          task.error = null;
          break;
        } catch (exc: any) {
          task.error = exc;
          console.warn(`Task '${name}' attempt ${attempt} failed: ${exc.message}`);
          if (attempt === task.retries) {
            console.error(`Task '${name}' exhausted retries`);
            throw exc;
          }
          await new Promise((r) => setTimeout(r, 500 * attempt));
        }
      }
      console.log(`Task '${name}' completed`);
    }
    const results: Record<string, any> = {};
    for (const [name, task] of Object.entries(this.tasks)) {
      results[name] = task.result;
    }
    return results;
  }
}

function fetchData(): { title: string; content: string } {
  return {
    title: "AI Agent Engineering",
    content: "Workflow orchestration is essential for reliable agent systems.",
  };
}

function makeAnalyzeSentiment(client: OpenAIClient | null) {
  return async (deps: { fetch: { title: string; content: string } }) => {
    const text = deps.fetch.content;
    if (!client) {
      console.log("No LLM available; using deterministic sentiment fallback");
      return "positive";
    }
    const messages: ChatMessage[] = [
      {
        role: "system",
        content:
          "Classify sentiment as exactly one word: positive, negative, or neutral.",
      },
      { role: "user", content: text },
    ];
    const resp = await client.chatCompletion({
      messages,
      temperature: 0.0,
      max_tokens: 10,
    });
    return resp.choices[0].message.content.trim().toLowerCase();
  };
}

function makeFlakyQualityCheck() {
  let callCount = 0;
  return (deps: { sentiment: string }) => {
    callCount += 1;
    if (callCount < 3) {
      throw new Error(`Quality check service unavailable (attempt ${callCount})`);
    }
    return `quality_ok (${deps.sentiment})`;
  };
}

function makeSummarize(client: OpenAIClient | null) {
  return async (deps: { fetch: { title: string; content: string }; sentiment: string }) => {
    if (!client) {
      console.log("No LLM available; using deterministic summary fallback");
      return `Summary: '${deps.fetch.title}' has ${deps.sentiment} sentiment.`;
    }
    const prompt =
      `Title: ${deps.fetch.title}\n` +
      `Content: ${deps.fetch.content}\n` +
      `Sentiment: ${deps.sentiment}\n` +
      "Write a one-sentence summary.";
    const resp = await client.chatCompletion({
      messages: [{ role: "user", content: prompt }],
      temperature: 0.0,
      max_tokens: 60,
    });
    return resp.choices[0].message.content.trim();
  };
}

async function main() {
  let client: OpenAIClient | null = null;
  try {
    client = new OpenAIClient();
  } catch (exc: any) {
    console.warn("LLM client disabled:", exc.message);
  }

  const engine = new WorkflowEngine();
  engine.addTask(new Task("fetch", fetchData));
  engine.addTask(
    new Task("sentiment", makeAnalyzeSentiment(client), ["fetch"])
  );
  engine.addTask(
    new Task("quality", makeFlakyQualityCheck(), ["sentiment"], 5)
  );
  engine.addTask(
    new Task("summary", makeSummarize(client), ["fetch", "sentiment"])
  );

  console.log("Starting DAG workflow...");
  const results = await engine.run();
  console.log("\nFinal results:");
  console.log(JSON.stringify(results, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
