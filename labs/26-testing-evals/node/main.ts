import fs from "node:fs";
import path from "node:path";
import { OpenAIClient } from "../../../shared/config/openai_client";

interface TestCase {
  id: string;
  input: string;
  reference: string;
  expected_keywords?: string[];
  expect_json?: boolean;
  max_latency_ms?: number;
}

function loadTestCases(): TestCase[] {
  return JSON.parse(fs.readFileSync(path.join(__dirname, "test_cases.json"), "utf8"));
}

function ruleCheck(answer: string, testCase: TestCase, latencyMs: number) {
  const checks: Record<string, { passed: boolean; [key: string]: any }> = {};

  const keywords = testCase.expected_keywords || [];
  const missing = keywords.filter((kw) => !answer.toLowerCase().includes(kw.toLowerCase()));
  checks.keywords = { passed: missing.length === 0, missing };

  if (testCase.expect_json) {
    try {
      JSON.parse(answer);
      checks.json = { passed: true };
    } catch (err) {
      checks.json = { passed: false, error: String(err) };
    }
  }

  if (testCase.max_latency_ms !== undefined) {
    checks.latency = { passed: latencyMs <= testCase.max_latency_ms, latencyMs };
  }

  const allPassed = Object.values(checks).every((c) => c.passed);
  return { passed: allPassed, checks };
}

async function llmJudge(client: OpenAIClient, answer: string, reference: string) {
  const prompt = `Rate how well the following answer matches the reference answer.\nAnswer: ${answer}\nReference: ${reference}\nRespond with JSON only: {"score": 1-10, "reason": "..."}`;
  const response = await client.chatCompletion({
    messages: [{ role: "user", content: prompt }],
    response_format: { type: "json_object" },
    max_tokens: 200,
    temperature: 0,
  });
  try {
    return JSON.parse(client.extractMessage(response).content || "{}");
  } catch (err) {
    return { score: 0, reason: `Failed to parse judge response: ${err}` };
  }
}

async function runAgent(client: OpenAIClient, question: string) {
  const response = await client.chatCompletion({
    messages: [{ role: "user", content: question }],
    max_tokens: 200,
    temperature: 0,
  });
  return client.extractMessage(response).content || "";
}

async function evaluate(client: OpenAIClient, cases: TestCase[]) {
  const results = [];
  for (const testCase of cases) {
    const start = performance.now();
    const answer = await runAgent(client, testCase.input);
    const latencyMs = performance.now() - start;

    const ruleResult = ruleCheck(answer, testCase, latencyMs);
    const judgeResult = await llmJudge(client, answer, testCase.reference);

    results.push({
      id: testCase.id,
      input: testCase.input,
      answer,
      rulePassed: ruleResult.passed,
      ruleDetails: ruleResult.checks,
      judgeScore: judgeResult.score,
      judgeReason: judgeResult.reason,
    });
  }

  const total = results.length;
  const rulePassed = results.filter((r) => r.rulePassed).length;
  const scores = results.map((r) => r.judgeScore).filter((s) => typeof s === "number") as number[];
  const avgScore = scores.length ? scores.reduce((a, b) => a + b, 0) / scores.length : 0;

  return {
    total,
    rulePassRate: total ? rulePassed / total : 0,
    averageJudgeScore: avgScore,
    cases: results,
  };
}

async function main() {
  const client = new OpenAIClient();
  const cases = loadTestCases();
  const report = await evaluate(client, cases);
  console.log(JSON.stringify(report, null, 2));
  console.log(`\nRule pass rate: ${(report.rulePassRate * 100).toFixed(0)}%`);
  console.log(`Average judge score: ${report.averageJudgeScore.toFixed(1)}/10`);
}

main().catch(console.error);
