import { OpenAIClient } from "../../../shared/config/openai_client";

async function main() {
  try {
    const client = new OpenAIClient();
    const response = await client.chatCompletion({
      messages: [
        { role: "system", content: "You are a concise assistant." },
        { role: "user", content: "Explain what an AI agent is in one sentence." },
      ],
      max_tokens: 80,
    });

    const message = client.extractMessage(response);
    console.log("Assistant:", message.content);
    console.log("Model: unknown");
    console.log("Finish reason:", response.choices[0].finish_reason);
    console.log("Usage:", JSON.stringify(response.usage, null, 2));
  } catch (err) {
    console.error("Request failed:", err);
    process.exit(1);
  }
}

main();
