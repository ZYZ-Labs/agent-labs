import { OpenAIClient, ChatMessage } from "../../../shared/config/openai_client";

const SCHEMA = {
  type: "object",
  properties: {
    service_name: { type: "string" },
    port: { type: "integer", minimum: 1024, maximum: 65535 },
    replicas: { type: "integer", minimum: 1, maximum: 10 },
    env: { type: "array", items: { type: "string" } },
  },
  required: ["service_name", "port", "replicas"],
};

function validateConfig(config: any): string | null {
  if (typeof config !== "object" || config === null) {
    return "Config must be a JSON object.";
  }
  for (const key of ["service_name", "port", "replicas"]) {
    if (!(key in config)) return `Missing required field: ${key}`;
  }
  if (!(1024 <= config.port && config.port <= 65535)) {
    return "port must be between 1024 and 65535.";
  }
  if (!(1 <= config.replicas && config.replicas <= 10)) {
    return "replicas must be between 1 and 10.";
  }
  return null;
}

async function generateWithRecovery(
  client: OpenAIClient,
  request: string,
  maxRetries = 3
): Promise<any> {
  const messages: ChatMessage[] = [
    {
      role: "system",
      content:
        "You are a configuration generator. " +
        `Return only valid JSON matching this schema: ${JSON.stringify(SCHEMA)}. ` +
        "No markdown, no explanation.",
    },
    { role: "user", content: request },
  ];

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    console.log(`\n--- Attempt ${attempt} ---`);
    const response = await client.chatCompletion({
      messages,
      temperature: 0.2,
      max_tokens: 300,
    });
    const raw = response.choices[0].message.content;
    console.log("Raw output:", raw);

    let parsed: any;
    try {
      parsed = JSON.parse(raw);
    } catch (exc: any) {
      const error = `Invalid JSON: ${exc.message}`;
      console.log(error);
      messages.push({ role: "assistant", content: raw });
      messages.push({
        role: "user",
        content: `That was not valid JSON. ${error} Please retry.`,
      });
      continue;
    }

    const error = validateConfig(parsed);
    if (error === null) return parsed;

    console.log("Validation error:", error);
    messages.push({ role: "assistant", content: raw });
    messages.push({
      role: "user",
      content: `Validation failed: ${error}. Fix the JSON and retry.`,
    });
  }

  throw new Error("Failed to generate valid config after max retries.");
}

async function main() {
  const client = new OpenAIClient();
  const request =
    "Create a config for a payment-api service on port 8080 with 3 replicas and env vars LOG_LEVEL=info,DB_URL=postgres.";
  const config = await generateWithRecovery(client, request);
  console.log("\nFinal valid config:");
  console.log(JSON.stringify(config, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
