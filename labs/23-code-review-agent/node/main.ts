import fs from "fs";
import path from "path";
import { spawnSync } from "child_process";
import { OpenAIClient } from "../../../shared/config/openai_client";

function findRuff(): string | null {
  const candidates = ["ruff", "ruff.exe"];
  for (const bin of candidates) {
    const result = spawnSync("which", [bin], { encoding: "utf8" });
    if (result.status === 0 && result.stdout.trim()) return result.stdout.trim();
  }
  return null;
}

function runSyntaxCheck(filePath: string): { ok: boolean; issues: string[] } {
  const result = spawnSync("python", ["-m", "py_compile", filePath], {
    encoding: "utf8",
  });
  if (result.status === 0) return { ok: true, issues: [] };
  return { ok: false, issues: [`Syntax error: ${result.stderr || result.stdout}`] };
}

function runRuff(filePath: string): { ok: boolean; issues: string[] } {
  const ruffBin = findRuff();
  if (!ruffBin) {
    console.warn("ruff not installed; skipping style check");
    return { ok: true, issues: ["ruff not installed"] };
  }
  const result = spawnSync(ruffBin, ["check", filePath, "--output-format", "json"], {
    encoding: "utf8",
  });
  const issues: string[] = [];
  if (result.stdout.trim()) {
    try {
      const parsed = JSON.parse(result.stdout);
      for (const item of parsed) {
        issues.push(
          `Line ${item.location?.row}: ${item.code} - ${item.message}`
        );
      }
    } catch {
      issues.push(result.stdout.trim());
    }
  }
  return { ok: issues.length === 0, issues };
}

async function llmReview(
  filePath: string,
  source: string,
  checks: any,
  client: OpenAIClient | null
): Promise<string> {
  if (!client) return "LLM review skipped (no API key). Manual review recommended.";

  const prompt = `Review the following Python file for style, bugs, and maintainability.

File: ${path.basename(filePath)}

Static checks:
- Syntax OK: ${checks.syntax.ok}
- Style issues: ${checks.ruff.issues}

Source code:
\`\`\`python
${source}
\`\`\`

Provide a concise review with:
1. Critical issues
2. Suggestions
3. Overall verdict (OK / Needs work).`;

  const resp = await client.chatCompletion({
    messages: [{ role: "user", content: prompt }],
    temperature: 0.2,
    max_tokens: 400,
  });
  return resp.choices[0].message.content.trim();
}

async function reviewFile(filePath: string, client: OpenAIClient | null): Promise<any> {
  console.log(`Reviewing ${filePath}`);
  const source = fs.readFileSync(filePath, "utf8");
  const checks = {
    syntax: runSyntaxCheck(filePath),
    ruff: runRuff(filePath),
  };
  const review = await llmReview(filePath, source, checks, client);
  const ruffIssues = checks.ruff.issues.filter((i) => i !== "ruff not installed");
  return {
    file: filePath,
    checks,
    llm_review: review,
    verdict:
      checks.syntax.ok && ruffIssues.length === 0 ? "ok" : "needs_work",
  };
}

async function main() {
  let client: OpenAIClient | null = null;
  try {
    client = new OpenAIClient();
  } catch (exc: any) {
    console.warn("LLM client disabled:", exc.message);
  }

  const target = process.argv[2]
    ? path.resolve(process.argv[2])
    : path.resolve("sample_code.py");

  if (!fs.existsSync(target)) {
    console.error("File not found:", target);
    process.exit(1);
  }

  const report = await reviewFile(target, client);
  console.log(JSON.stringify(report, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
