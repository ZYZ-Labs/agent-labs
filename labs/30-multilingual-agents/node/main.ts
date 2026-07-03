import { OpenAIClient } from "../../../shared/config/openai_client";

const SUPPORTED_LANGUAGES = ["English", "Chinese", "Japanese", "Spanish", "French", "German"];

async function detectLanguage(client: OpenAIClient, text: string): Promise<string> {
  const response = await client.chatCompletion({
    messages: [
      {
        role: "system",
        content:
          "You are a language detector. Reply with only the language name in English (e.g., 'English', 'Chinese').",
      },
      { role: "user", content: `Detect the language of this text:\n${text}` },
    ],
    max_tokens: 20,
    temperature: 0.0,
  });
  return client.extractMessage(response).content?.trim() || "Unknown";
}

async function translateGreeting(
  client: OpenAIClient,
  greeting: string,
  targetLang: string
): Promise<string> {
  const response = await client.chatCompletion({
    messages: [
      {
        role: "system",
        content: `Translate the user's text into ${targetLang}. Reply with the translation only.`,
      },
      { role: "user", content: greeting },
    ],
    max_tokens: 60,
    temperature: 0.0,
  });
  return client.extractMessage(response).content?.trim() || "";
}

async function main() {
  const client = new OpenAIClient();
  const greeting = process.env.GREETING || "Hello, how are you today?";

  console.log("Original:", greeting);

  const detected = await detectLanguage(client, greeting);
  console.log("Detected language:", detected);

  console.log("\nTranslations:");
  for (const lang of SUPPORTED_LANGUAGES.filter((l) => l !== detected)) {
    const translated = await translateGreeting(client, greeting, lang);
    console.log(`  ${lang}: ${translated}`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
