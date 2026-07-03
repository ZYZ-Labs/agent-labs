import { OpenAIClient, ChatCompletionOptions, ChatMessage } from "../../../shared/config/openai_client";

const defaultMessages: ChatMessage[] = [
  { role: "system", content: "You are a helpful coding assistant. Be concise." },
  {
    role: "user",
    content:
      "List three benefits of using state machines to model agent workflows. Answer in at most two sentences.",
  },
];

async function runCase(
  client: OpenAIClient,
  name: string,
  params: Partial<ChatCompletionOptions>,
  messages: ChatMessage[] = defaultMessages
) {
  console.log(`\n=== ${name} ===`);
  try {
    const response = await client.chatCompletion({ messages, ...params });
    response.choices.forEach((choice, idx) => {
      const prefix = response.choices.length > 1 ? `  Choice ${idx}:` : "  Output:";
      console.log(`${prefix} ${choice.message.content?.trim()}`);
    });
    console.log(`  Finish reason: ${response.choices[0].finish_reason}`);
    console.log(`  Usage: ${JSON.stringify(response.usage)}`);
  } catch (err) {
    console.error("  Error:", err);
  }
}

async function main() {
  const client = new OpenAIClient();

  await runCase(client, "Default", { max_tokens: 120 });
  await runCase(client, "High temperature (creative)", { temperature: 1.2, max_tokens: 120 });
  await runCase(client, "Low temperature (deterministic)", { temperature: 0, max_tokens: 120 });
  await runCase(client, "top_p nucleus sampling", { top_p: 0.3, max_tokens: 120 });
  await runCase(client, "Frequency penalty", { frequency_penalty: 1, max_tokens: 120 });
  await runCase(client, "Presence penalty", { presence_penalty: 1, max_tokens: 120 });
  await runCase(client, "Stop sequence", { stop: ["."], max_tokens: 120 });
  await runCase(client, "Seed for reproducibility", { seed: 42, temperature: 0, max_tokens: 120 });
  await runCase(client, "JSON response format", {
    response_format: { type: "json_object" },
    messages: [
      ...defaultMessages,
      { role: "user", content: "Return the answer as JSON with keys: summary, benefits." },
    ],
  });
  await runCase(client, "Multiple choices n=3", { n: 3, temperature: 1, max_tokens: 60 });
}

main().catch(console.error);
