import { OpenAIClient } from "../../../shared/config/openai_client";

const MODEL_PRICING: Record<string, { input: number; output: number }> = {
  "gpt-4o": { input: 5.0, output: 15.0 },
  "gpt-4o-mini": { input: 0.15, output: 0.6 },
  "gpt-3.5-turbo": { input: 0.5, output: 1.5 },
};

function estimateCost(model: string, promptTokens: number, completionTokens: number) {
  const pricing = MODEL_PRICING[model] || { input: 0, output: 0 };
  return (promptTokens * pricing.input + completionTokens * pricing.output) / 1_000_000;
}

function compareCosts(promptTokens: number, completionTokens: number) {
  return Object.entries(MODEL_PRICING).map(([model]) => ({
    model,
    estimatedCostUsd: +estimateCost(model, promptTokens, completionTokens).toFixed(6),
  }));
}

function chooseModel(taskDescription: string) {
  const desc = taskDescription.toLowerCase();
  const complexSignals = ["architecture", "design doc", "refactor", "complex", "multistep", "review"];
  return complexSignals.some((s) => desc.includes(s)) ? "gpt-4o" : "gpt-4o-mini";
}

function countTokens(text: string) {
  // Fallback rough estimate: ~4 chars per token or whitespace split.
  return Math.ceil(text.length / 4) || Math.ceil(text.split(/\s+/).length);
}

async function runCachedPromptAgent(client: OpenAIClient, userMessage: string) {
  const systemPrefix = "You are a concise coding assistant. Always answer in one sentence.";
  const fullPrompt = `${systemPrefix}\n${userMessage}`;
  const fullTokens = countTokens(fullPrompt);
  const userTokens = countTokens(userMessage);

  const response = await client.chatCompletion({
    messages: [
      { role: "system", content: systemPrefix },
      { role: "user", content: userMessage },
    ],
    max_tokens: 200,
    temperature: 0,
  });
  const content = client.extractMessage(response).content || "";
  const completionTokens = response.usage?.completion_tokens ?? countTokens(content);

  return {
    model: client.config.model,
    userMessage,
    fullPromptTokens: fullTokens,
    newInputTokens: userTokens,
    completionTokens,
    estimatedCostCachedUsd: +estimateCost(client.config.model, userTokens, completionTokens).toFixed(6),
    estimatedCostUncachedUsd: +estimateCost(client.config.model, fullTokens, completionTokens).toFixed(6),
    response: content,
  };
}

async function main() {
  const client = new OpenAIClient();

  console.log("Cost comparison for 1000 input + 500 output tokens:");
  for (const row of compareCosts(1000, 500)) {
    console.log(`  ${row.model}: $${row.estimatedCostUsd.toFixed(6)}`);
  }

  const tasks = [
    "Summarize this paragraph in one sentence.",
    "Generate an architecture design doc for a payment gateway.",
    "Refactor this Python function to use async IO.",
  ];
  console.log("\nRouting decisions:");
  for (const task of tasks) {
    const model = chooseModel(task);
    console.log(`  [${model}] ${task}`);
  }

  const userMessage = "What is the capital of France?";
  console.log(`\nCached prompt example:\nUser: ${userMessage}`);
  const result = await runCachedPromptAgent(client, userMessage);
  console.log(JSON.stringify(result, null, 2));
}

main().catch(console.error);
