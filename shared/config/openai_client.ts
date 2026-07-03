// Shared OpenAI-compatible client wrapper for Node/TypeScript labs.
// Reads configuration from environment variables.

import * as dotenv from "dotenv";
import fetch from "node-fetch";

export interface OpenAIConfig {
  apiKey: string;
  baseUrl: string;
  model: string;
  logLevel: string;
}

export interface ChatMessage {
  role: "system" | "user" | "assistant" | "tool";
  content: string;
  name?: string;
  tool_call_id?: string;
}

export interface ChatCompletionOptions {
  model?: string;
  messages: ChatMessage[];
  temperature?: number;
  max_tokens?: number;
  top_p?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
  stop?: string[] | string;
  seed?: number;
  response_format?: Record<string, string>;
  tools?: unknown[];
  tool_choice?: string | Record<string, unknown>;
  stream?: boolean;
  n?: number;
  extra_body?: Record<string, unknown>;
}

export interface ChatCompletionResponse {
  choices: Array<{
    message: ChatMessage & { tool_calls?: unknown[] };
    finish_reason: string;
  }>;
  usage?: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };
}

export function loadConfig(path?: string): OpenAIConfig {
  dotenv.config({ path: path || ".env" });
  return {
    apiKey: process.env.OPENAI_API_KEY || "",
    baseUrl: (process.env.OPENAI_BASE_URL || "https://api.openai.com/v1").replace(/\/$/, ""),
    model: process.env.OPENAI_MODEL || "gpt-4o-mini",
    logLevel: process.env.LOG_LEVEL || "INFO",
  };
}

export class OpenAIClient {
  public readonly config: OpenAIConfig;
  private timeoutMs: number;
  private maxRetries: number;

  constructor(options?: Partial<OpenAIConfig & { timeoutMs?: number; maxRetries?: number }>) {
    const env = loadConfig();
    this.config = {
      apiKey: options?.apiKey ?? env.apiKey,
      baseUrl: options?.baseUrl ?? env.baseUrl,
      model: options?.model ?? env.model,
      logLevel: options?.logLevel ?? env.logLevel,
    };
    this.timeoutMs = options?.timeoutMs ?? 60000;
    this.maxRetries = options?.maxRetries ?? 3;
    if (!this.config.apiKey && !this.config.baseUrl.startsWith("http://localhost")) {
      throw new Error("OPENAI_API_KEY is required for non-local endpoints");
    }
  }

  async chatCompletion(options: ChatCompletionOptions): Promise<ChatCompletionResponse> {
    const url = `${this.config.baseUrl}/chat/completions`;
    const payload: Record<string, unknown> = {
      model: options.model ?? this.config.model,
      messages: options.messages,
      stream: options.stream ?? false,
    };
    if (options.temperature !== undefined) payload.temperature = options.temperature;
    if (options.max_tokens !== undefined) payload.max_tokens = options.max_tokens;
    if (options.top_p !== undefined) payload.top_p = options.top_p;
    if (options.frequency_penalty !== undefined) payload.frequency_penalty = options.frequency_penalty;
    if (options.presence_penalty !== undefined) payload.presence_penalty = options.presence_penalty;
    if (options.stop !== undefined) payload.stop = options.stop;
    if (options.seed !== undefined) payload.seed = options.seed;
    if (options.response_format !== undefined) payload.response_format = options.response_format;
    if (options.tools !== undefined) payload.tools = options.tools;
    if (options.tool_choice !== undefined) payload.tool_choice = options.tool_choice;
    if (options.n !== undefined) payload.n = options.n;
    if (options.extra_body) Object.assign(payload, options.extra_body);

    if (this.config.logLevel === "DEBUG") {
      console.debug("POST", url, JSON.stringify(payload, null, 2));
    }

    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Authorization: `Bearer ${this.config.apiKey}`,
    };

    for (let attempt = 1; attempt <= this.maxRetries; attempt++) {
      try {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), this.timeoutMs);
        const response = await fetch(url, {
          method: "POST",
          headers,
          body: JSON.stringify(payload),
          signal: controller.signal,
        });
        clearTimeout(timer);
        if (!response.ok) {
          const text = await response.text();
          throw new Error(`HTTP ${response.status}: ${text}`);
        }
        return (await response.json()) as ChatCompletionResponse;
      } catch (err) {
        console.warn(`Request attempt ${attempt} failed:`, err);
        if (attempt === this.maxRetries) throw err;
      }
    }
    throw new Error("Unreachable");
  }

  extractMessage(response: ChatCompletionResponse): ChatCompletionResponse["choices"][0]["message"] {
    return response.choices[0].message;
  }
}

async function main() {
  const client = new OpenAIClient();
  const resp = await client.chatCompletion({
    messages: [{ role: "user", content: "Say hello in one word." }],
    max_tokens: 10,
  });
  console.log(resp.choices[0].message.content);
}

if (require.main === module) {
  main().catch(console.error);
}
