import fs from "fs";
import path from "path";
import { OpenAIClient } from "../../../shared/config/openai_client";

const REVIEW_PROMPT = `You are a meticulous code reviewer. Review the following Python source file and return ONLY a JSON object.

Rules for the JSON object:
- Top-level keys must be exactly: "security", "style", "logic".
- Each value is a list of findings. An empty list is allowed.
- Each finding is an object with these keys:
  - "severity": one of "HIGH", "MEDIUM", "LOW".
  - "line": integer line number, or null if not applicable.
  - "message": concise description of the issue.
  - "suggestion": concrete recommendation to fix it.

Be strict but fair. Focus on real issues, not nitpicks.

\`\`\`python
{code}
\`\`\`
`;

const CATEGORIES = ["security", "style", "logic"];
const SEVERITY_ORDER: Record<string, number> = { HIGH: 0, MEDIUM: 1, LOW: 2 };

function readSourceFile(filePath: string): string {
  if (!fs.existsSync(filePath)) throw new Error(`Source file not found: ${filePath}`);
  const stat = fs.statSync(filePath);
  if (!stat.isFile()) throw new Error(`Path is not a file: ${filePath}`);
  return fs.readFileSync(filePath, "utf8");
}

async function reviewCode(client: OpenAIClient, source: string): Promise<any> {
  const prompt = REVIEW_PROMPT.replace("{code}", source);
  console.log(`Sending code review prompt (${source.length} chars of code).`);

  const response = await client.chatCompletion({
    messages: [{ role: "user", content: prompt }],
    temperature: 0.2,
    max_tokens: 1200,
    response_format: { type: "json_object" },
  });

  const content = response.choices[0].message.content;
  try {
    return JSON.parse(content);
  } catch (exc: any) {
    throw new Error(`Model returned invalid JSON: ${exc.message}\nRaw content:\n${content}`);
  }
}

function validateReport(report: any): void {
  if (typeof report !== "object" || report === null) {
    throw new Error("Review report is not a JSON object.");
  }
  const missing = CATEGORIES.filter((cat) => !(cat in report));
  if (missing.length > 0) {
    throw new Error(`Review report missing required categories: ${missing.join(", ")}`);
  }
  for (const category of CATEGORIES) {
    if (!Array.isArray(report[category])) {
      throw new Error(`Category '${category}' must be a list.`);
    }
    for (let idx = 0; idx < report[category].length; idx++) {
      const finding = report[category][idx];
      if (typeof finding !== "object" || finding === null) {
        throw new Error(`Finding ${idx} in '${category}' is not an object.`);
      }
      for (const key of ["severity", "message", "suggestion"]) {
        if (!(key in finding)) {
          throw new Error(`Finding ${idx} in '${category}' missing key '${key}'.`);
        }
      }
      if (!(finding.severity in SEVERITY_ORDER)) {
        throw new Error(
          `Finding ${idx} in '${category}' has invalid severity '${finding.severity}'.`
        );
      }
    }
  }
}

function printReport(filePath: string, lineCount: number, report: any): void {
  const categoriesPresent = CATEGORIES.filter((cat) => report[cat]?.length > 0);

  console.log(`\nReview: ${filePath}`);
  console.log(`Lines: ${lineCount}`);
  console.log(`Categories: ${categoriesPresent.join(", ") || "none with findings"}`);

  let total = 0;
  const counts: Record<string, number> = { HIGH: 0, MEDIUM: 0, LOW: 0 };

  for (const category of CATEGORIES) {
    const findings: any[] = report[category] || [];
    if (findings.length === 0) continue;
    console.log(`\n${category.toUpperCase()}`);
    for (const finding of findings.sort(
      (a, b) => (SEVERITY_ORDER[a.severity] ?? 99) - (SEVERITY_ORDER[b.severity] ?? 99)
    )) {
      total += 1;
      counts[finding.severity] += 1;
      const line = finding.line;
      const lineInfo = line != null ? ` at line ${line}` : "";
      console.log(`  - ${finding.severity}${lineInfo}: ${finding.message}`);
      console.log(`    Suggestion: ${finding.suggestion}`);
    }
  }

  console.log(
    `\nSummary: ${total} issue(s) found. ${counts.HIGH} high, ${counts.MEDIUM} medium, ${counts.LOW} low.`
  );
}

async function main() {
  const client = new OpenAIClient();

  const target = process.argv[2]
    ? path.resolve(process.argv[2])
    : path.join(__dirname, "sample_code.py");

  let source: string;
  try {
    source = readSourceFile(target);
  } catch (exc: any) {
    console.error(exc.message);
    process.exit(1);
  }

  let report: any;
  try {
    report = await reviewCode(client, source);
    validateReport(report);
  } catch (exc: any) {
    console.error("Review failed:", exc.message);
    process.exit(1);
  }

  printReport(target, source.split("\n").length, report);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
