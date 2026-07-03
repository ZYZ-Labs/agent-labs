import { OpenAIClient, ChatMessage } from "../../../shared/config/openai_client";

const EVENT_SCHEMA = {
  type: "object",
  properties: {
    name: { type: "string" },
    date: { type: "string" },
    location: { type: "string" },
    participants: { type: "array", items: { type: "string" } },
  },
  required: ["name", "date"],
};

const USER_PROMPT = `
Extract the event details from this message as JSON:

"Join us for the AI Engineering Meetup on 2025-09-15 at the Shenzhen Hub.
Attendees: Alice, Bob, and Carol."
`.trim();

function validateEvent(data: any): any {
  if (typeof data !== "object" || data === null) {
    throw new Error("Parsed JSON is not an object");
  }
  const missing = EVENT_SCHEMA.required.filter((f) => !(f in data));
  if (missing.length > 0) {
    throw new Error(`Missing required fields: ${missing.join(", ")}`);
  }
  for (const [key, value] of Object.entries(data)) {
    const spec = (EVENT_SCHEMA.properties as any)[key];
    if (spec?.type === "array" && !Array.isArray(value)) {
      throw new Error(`Field '${key}' should be an array`);
    }
  }
  return data;
}

async function extractWithJsonMode(client: OpenAIClient): Promise<any> {
  const response = await client.chatCompletion({
    messages: [
      {
        role: "system",
        content:
          "You are a helpful parser. " +
          "Return ONLY a JSON object with keys: name, date, location, participants.",
      },
      { role: "user", content: USER_PROMPT },
    ],
    response_format: { type: "json_object" },
    temperature: 0.0,
    max_tokens: 300,
  });
  const raw = response.choices[0].message.content;
  console.log("\n[JSON mode] raw output:");
  console.log(raw);
  return validateEvent(JSON.parse(raw));
}

async function extractWithFunctionCall(client: OpenAIClient): Promise<any> {
  const tool = {
    type: "function",
    function: {
      name: "extract_event",
      description: "Extract event details from user text.",
      parameters: EVENT_SCHEMA,
    },
  };
  const response = await client.chatCompletion({
    messages: [
      { role: "system", content: "Use the extract_event tool." },
      { role: "user", content: USER_PROMPT },
    ],
    tools: [tool as any],
    tool_choice: { type: "function", function: { name: "extract_event" } } as any,
    temperature: 0.0,
    max_tokens: 300,
  });
  const message = response.choices[0].message;
  const toolCalls = (message.tool_calls as any[]) || [];
  if (toolCalls.length === 0) {
    throw new Error("Model did not call the extract_event tool");
  }
  const raw = toolCalls[0].function.arguments;
  console.log("\n[Function call] raw arguments:");
  console.log(raw);
  return validateEvent(JSON.parse(raw));
}

async function runMode(
  label: string,
  extractor: (client: OpenAIClient) => Promise<any>,
  client: OpenAIClient
): Promise<{ ok: boolean; result?: any; error?: string }> {
  console.log(`\n${"=".repeat(40)}\nMode: ${label}\n${"=".repeat(40)}`);
  try {
    const result = await extractor(client);
    console.log(`\n[${label}] parsed event:`);
    console.log(JSON.stringify(result, null, 2));
    return { ok: true, result };
  } catch (exc: any) {
    console.log(`\n[${label}] FAILED: ${exc.message}`);
    return { ok: false, error: exc.message };
  }
}

async function main() {
  const client = new OpenAIClient();

  const jsonMode = await runMode("JSON mode", extractWithJsonMode, client);
  const functionMode = await runMode("Function calling", extractWithFunctionCall, client);

  console.log("\n" + "=".repeat(40));
  console.log("Summary");
  console.log("=".repeat(40));
  console.log(`JSON mode ok: ${jsonMode.ok}`);
  console.log(`Function call ok: ${functionMode.ok}`);

  if (jsonMode.ok && functionMode.ok) {
    const j = jsonMode.result;
    const f = functionMode.result;
    console.log(
      "\nBoth approaches returned valid events. " +
        "Function calling gives you an explicit schema contract; " +
        "JSON mode is simpler when you only need a shaped text response."
    );
    console.log(
      "Names match:",
      j.name === f.name && j.date === f.date
    );
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
