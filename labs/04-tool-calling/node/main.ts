import { OpenAIClient, ChatMessage } from "../../../shared/config/openai_client";

const TOOLS = [
  {
    type: "function",
    function: {
      name: "get_weather",
      description: "Get current weather for a city.",
      parameters: {
        type: "object",
        properties: {
          city: { type: "string", description: "City name" },
        },
        required: ["city"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "search_notes",
      description: "Search project notes by keyword.",
      parameters: {
        type: "object",
        properties: {
          query: { type: "string", description: "Search keyword" },
        },
        required: ["query"],
      },
    },
  },
];

const NOTES = [
  {
    title: "MCP design",
    content: "MCP uses Resources, Tools, Prompts, and Sampling primitives.",
  },
  {
    title: "LSP basics",
    content: "LSP speaks JSON-RPC over stdio or sockets.",
  },
  {
    title: "Agent memory",
    content: "Short-term memory lives in the context window; long-term in vectors.",
  },
];

function getWeather(city: string): string {
  return JSON.stringify({ city, temperature_c: 22, condition: "sunny" });
}

function searchNotes(query: string): string {
  const results = NOTES.filter(
    (n) =>
      n.title.toLowerCase().includes(query.toLowerCase()) ||
      n.content.toLowerCase().includes(query.toLowerCase())
  );
  return JSON.stringify(results);
}

const TOOL_FUNCTIONS: Record<string, (args: any) => string> = {
  get_weather: getWeather,
  search_notes: searchNotes,
};

interface ToolCall {
  id: string;
  function: {
    name: string;
    arguments: string;
  };
}

async function runToolAgent(
  client: OpenAIClient,
  userMessage: string,
  maxIterations = 5
): Promise<string> {
  const messages: ChatMessage[] = [{ role: "user", content: userMessage }];

  for (let i = 0; i < maxIterations; i++) {
    const response = await client.chatCompletion({
      messages,
      tools: TOOLS as any,
      tool_choice: "auto",
      temperature: 0.0,
      max_tokens: 300,
    });
    const message = response.choices[0].message;
    messages.push(message);

    const toolCalls = (message.tool_calls as ToolCall[] | undefined) || [];
    if (toolCalls.length === 0) {
      return message.content;
    }

    for (const toolCall of toolCalls) {
      const name = toolCall.function.name;
      const args = JSON.parse(toolCall.function.arguments);
      console.log(`[Tool call] ${name}(${JSON.stringify(args)})`);
      const func = TOOL_FUNCTIONS[name];
      const result = func
        ? func(args)
        : JSON.stringify({ error: `unknown tool ${name}` });
      messages.push({
        role: "tool",
        tool_call_id: toolCall.id,
        name,
        content: result,
      });
      console.log(`[Tool result] ${result}`);
    }
  }

  return "Reached max iterations.";
}

async function main() {
  const client = new OpenAIClient();
  const question = "What's the weather in Shanghai? Also, find me notes about MCP.";
  console.log("User:", question);
  const answer = await runToolAgent(client, question);
  console.log("\nAssistant:", answer);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
