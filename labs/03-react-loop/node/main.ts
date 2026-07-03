import { OpenAIClient, ChatMessage } from "../../../shared/config/openai_client";

const SYSTEM_PROMPT = `You are a helpful assistant that solves problems step by step.
You must follow this format exactly:

Thought: describe your reasoning
Action: tool_name(arg1, arg2, ...)
Observation: the result of the action (provided by the system)
...
Final Answer: the final answer

Available tools:
- calculator(expression: str) - evaluates a Python arithmetic expression safely
- finish(answer: str) - use when you have the final answer
`;

function calculator(expression: string): string {
  const allowed = /^[0-9+\-*/(). ]+$/;
  if (!allowed.test(expression)) {
    return "Error: invalid characters";
  }
  try {
    // eslint-disable-next-line no-eval
    return String(eval(expression));
  } catch (exc: any) {
    return `Error: ${exc}`;
  }
}

function parseAction(text: string): { tool: string; args: string[] } | null {
  const match = text.match(/Action:\s*(\w+)\((.*)\)/s);
  if (!match) return null;
  const tool = match[1];
  const argsStr = match[2];
  const args = argsStr
    .split(",")
    .map((a) => a.trim().replace(/^["']|["']$/g, ""))
    .filter((a) => a.length > 0);
  return { tool, args };
}

async function runReact(client: OpenAIClient, question: string, maxSteps = 10): Promise<string> {
  const messages: ChatMessage[] = [
    { role: "system", content: SYSTEM_PROMPT },
    { role: "user", content: question },
  ];

  for (let step = 0; step < maxSteps; step++) {
    const response = await client.chatCompletion({
      messages,
      temperature: 0.0,
      max_tokens: 200,
    });
    const text = response.choices[0].message.content;
    console.log(`\n--- Step ${step + 1} ---`);
    console.log(text);

    if (text.includes("Final Answer:")) {
      return text.split("Final Answer:", 2)[1].trim();
    }

    const parsed = parseAction(text);
    let observation: string;
    if (!parsed) {
      observation =
        "Observation: I did not understand the action. Please use 'Action: tool_name(args)'.";
    } else {
      const { tool, args } = parsed;
      if (tool === "calculator" && args.length > 0) {
        const result = calculator(args[0]);
        observation = `Observation: ${result}`;
      } else if (tool === "finish" && args.length > 0) {
        return args[0];
      } else {
        observation = `Observation: unknown tool '${tool}'`;
      }
    }

    console.log(observation);
    messages.push({ role: "assistant", content: text });
    messages.push({ role: "user", content: observation });
  }

  return "Reached max steps without final answer.";
}

async function main() {
  const client = new OpenAIClient();
  const question = "What is (128 + 256) * 2 - 100?";
  console.log("Question:", question);
  const answer = await runReact(client, question);
  console.log("\nFinal Answer:", answer);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
