import { OpenAIClient, ChatMessage } from "../../../shared/config/openai_client";

const AGENTS: Record<
  string,
  { system: string; fallback: string }
> = {
  coding: {
    system:
      "You are a coding assistant. Answer the user's programming question with concise code and explanation.",
    fallback: "Use Python functions and add type hints for clarity.",
  },
  writing: {
    system:
      "You are a writing assistant. Improve clarity, grammar, and tone.",
    fallback: "Use short sentences and active voice.",
  },
  math: {
    system: "You are a math assistant. Solve the problem step by step.",
    fallback: "Break the problem into smaller equations.",
  },
};

class MultiAgentSystem {
  constructor(private client: OpenAIClient | null) {}

  async route(request: string): Promise<string[]> {
    if (!this.client) {
      console.log("No LLM; routing by keyword fallback");
      const lowered = request.toLowerCase();
      const topics: string[] = [];
      if (["code", "python", "function", "error"].some((k) => lowered.includes(k))) topics.push("coding");
      if (["write", "essay", "grammar", "draft"].some((k) => lowered.includes(k))) topics.push("writing");
      if (["math", "calculate", "equation", "sum"].some((k) => lowered.includes(k))) topics.push("math");
      return topics.length > 0 ? topics : ["writing"];
    }

    const messages: ChatMessage[] = [
      {
        role: "system",
        content:
          "You are a router. Given a user request, choose one or more specialist topics from: coding, writing, math. Reply with a JSON array of strings only, e.g. [\"coding\"].",
      },
      { role: "user", content: request },
    ];
    const resp = await this.client.chatCompletion({
      messages,
      temperature: 0.0,
      max_tokens: 50,
      response_format: { type: "json_object" },
    });
    const content = resp.choices[0].message.content;
    try {
      const parsed = JSON.parse(content);
      if (Array.isArray(parsed)) {
        return parsed.filter((t: string) => t in AGENTS);
      }
      if (typeof parsed === "object" && parsed !== null) {
        return (parsed.topics || []).filter((t: string) => t in AGENTS);
      }
    } catch {
      // ignore
    }
    return ["writing"];
  }

  async worker(topic: string, request: string): Promise<{ topic: string; answer: string }> {
    const cfg = AGENTS[topic];
    let answer: string;
    if (!this.client) {
      answer = cfg.fallback;
    } else {
      const messages: ChatMessage[] = [
        { role: "system", content: cfg.system },
        { role: "user", content: request },
      ];
      const resp = await this.client.chatCompletion({
        messages,
        temperature: 0.3,
        max_tokens: 200,
      });
      answer = resp.choices[0].message.content.trim();
    }
    return { topic, answer };
  }

  async aggregate(request: string, responses: { topic: string; answer: string }[]): Promise<string> {
    if (!this.client) {
      return responses.map((r) => `### ${r.topic}\n${r.answer}`).join("\n\n");
    }
    const combined = responses.map((r) => `### ${r.topic}\n${r.answer}`).join("\n\n");
    const messages: ChatMessage[] = [
      {
        role: "system",
        content:
          "You are an aggregator. Combine the specialist answers into a single coherent response.",
      },
      {
        role: "user",
        content: `User request: ${request}\n\nSpecialist answers:\n${combined}\n\nProvide a final answer.`,
      },
    ];
    const resp = await this.client.chatCompletion({
      messages,
      temperature: 0.3,
      max_tokens: 300,
    });
    return resp.choices[0].message.content.trim();
  }

  async run(request: string): Promise<{ topics: string[]; responses: { topic: string; answer: string }[]; final_answer: string }> {
    const topics = await this.route(request);
    console.log("Routed to:", topics.join(", "));
    const responses: { topic: string; answer: string }[] = [];
    for (const t of topics) {
      responses.push(await this.worker(t, request));
    }
    const final = await this.aggregate(request, responses);
    return { topics, responses, final_answer: final };
  }
}

async function main() {
  let client: OpenAIClient | null = null;
  try {
    client = new OpenAIClient();
  } catch (exc: any) {
    console.warn("LLM client disabled:", exc.message);
  }

  const system = new MultiAgentSystem(client);
  const request = "How do I write a Python function that retries a failing operation?";
  console.log("User request:", request);
  const result = await system.run(request);
  console.log("\nRouted to:", result.topics.join(", "));
  console.log("\nSpecialist answers:");
  for (const r of result.responses) {
    console.log(`  [${r.topic}] ${r.answer.slice(0, 200)}...`);
  }
  console.log("\nFinal aggregated answer:");
  console.log(result.final_answer);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
