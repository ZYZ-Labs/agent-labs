import 'dotenv/config';
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
  baseURL: process.env.OPENAI_BASE_URL || 'https://api.openai.com/v1',
});

const model = process.env.OPENAI_MODEL || 'gpt-4o-mini';

async function main() {
  const question = 'Say hello in one sentence.';
  console.log('User:', question);

  const response = await client.chat.completions.create({
    model,
    messages: [{ role: 'user', content: question }],
    max_tokens: 50,
    temperature: 0.0,
  });

  console.log('Assistant:', response.choices[0].message.content);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
