import { OpenAIClient } from "../../../shared/config/openai_client";

const TEST_CASES = [
  {
    input: "What port range is safe for user services?",
    expected_keywords: ["1024", "65535"],
    reference: "User services should use ports from 1024 to 65535.",
  },
  {
    input: "Explain idempotency in one sentence.",
    expected_keywords: ["same", "multiple", "result"],
    reference:
      "Idempotency means calling an operation multiple times produces the same result.",
  },
];

function ruleCheck(answer: string, keywords: string[]): { passed: boolean; missing: string[] } {
  const missing = keywords.filter((kw) => !answer.toLowerCase().includes(kw.toLowerCase()));
  return { passed: missing.length === 0, missing };
}

async function llmJudge(client: OpenAIClient, answer: string, reference: string): Promise<any> {
  const prompt = `Rate how well the following answer matches the reference answer.
Answer: ${answer}
Reference: ${reference}
Respond with JSON only: {"score": 1-10, "reason": "..."}`;
  const response = await client.chatCompletion({
    messages: [{ role: "user", content: prompt }],
    response_format: { type: "json_object" },
    max_tokens: 200,
    temperature: 0.0,
  });
  return JSON.parse(response.choices[0].message.content);
}

async function evaluateAgent(
  client: OpenAIClient,
  agentFn: (client: OpenAIClient, input: string) => Promise<string>
): Promise<any[]> {
  const results: any[] = [];
  for (const testCase of TEST_CASES) {
    const answer = await agentFn(client, testCase.input);
    const ruleResult = ruleCheck(answer, testCase.expected_keywords);
    const judgeResult = await llmJudge(client, answer, testCase.reference);
    results.push({
      input: testCase.input,
      answer,
      rule_passed: ruleResult.passed,
      missing_keywords: ruleResult.missing,
      judge_score: judgeResult.score,
      judge_reason: judgeResult.reason,
    });
  }
  return results;
}

async function simpleAgent(client: OpenAIClient, question: string): Promise<string> {
  const response = await client.chatCompletion({
    messages: [{ role: "user", content: question }],
    max_tokens: 200,
    temperature: 0.0,
  });
  return response.choices[0].message.content;
}

async function main() {
  const client = new OpenAIClient();
  const results = await evaluateAgent(client, simpleAgent);

  const passed = results.filter((r) => r.rule_passed).length;
  const total = results.length;
  const avgScore = results.reduce((sum, r) => sum + (r.judge_score || 0), 0) / total;

  console.log(JSON.stringify(results, null, 2));
  console.log(`\nRule checks passed: ${passed}/${total}`);
  console.log(`Average judge score: ${avgScore.toFixed(1)}/10`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
