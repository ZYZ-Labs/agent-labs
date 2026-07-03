import { randomUUID } from "node:crypto";
import { OpenAIClient, ChatCompletionResponse } from "../../../shared/config/openai_client";

interface SpanAttrs {
  [key: string]: any;
}

function logStructured(level: string, traceId: string, spanName: string, attrs: SpanAttrs) {
  console.log(JSON.stringify({ timestamp: new Date().toISOString(), level, trace_id: traceId, span_name: spanName, ...attrs }));
}

async function span<T>(name: string, traceId: string, attrs: SpanAttrs, fn: () => Promise<T>): Promise<T> {
  logStructured("INFO", traceId, name, { event: "span.start", ...attrs });
  const start = Date.now();
  try {
    return await fn();
  } finally {
    logStructured("INFO", traceId, name, { event: "span.end", duration_ms: Date.now() - start });
  }
}

function logUsage(traceId: string, response: ChatCompletionResponse) {
  logStructured("INFO", traceId, "usage", { event: "tokens.usage", ...response.usage });
}

async function runObservableAgent(client: OpenAIClient, userMessage: string) {
  const traceId = randomUUID();
  return span("agent.run", traceId, { input_length: userMessage.length }, async () => {
    const response = await span("llm.call", traceId, {}, () =>
      client.chatCompletion({
        messages: [{ role: "user", content: userMessage }],
        max_tokens: 200,
        temperature: 0,
      })
    );
    logUsage(traceId, response);

    const message = client.extractMessage(response);
    const content = message.content || "";
    const toolCalls = (message.tool_calls || []) as any[];
    for (const tc of toolCalls) {
      await span("tool.execute", traceId, { tool_name: tc.function.name }, async () => {
        // noop
      });
    }

    logStructured("INFO", traceId, "agent.run", { event: "agent.response", response: content });
    return content;
  });
}

async function main() {
  const client = new OpenAIClient();
  const question = "Explain observability in one sentence.";
  console.log(`User: ${question}`);
  const answer = await runObservableAgent(client, question);
  console.log(`Assistant: ${answer}`);
}

main().catch(console.error);
