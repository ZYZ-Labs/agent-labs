import fs from "fs";
import path from "path";
import { OpenAIClient } from "../../../shared/config/openai_client";

const REQUIREMENT =
  "Build an API for a task manager. Users can create a task with a title and " +
  "optional description, list all tasks, and mark a task as complete. " +
  "Store tasks in memory only.";

const PROMPT_VERSION = process.env.PROMPT_VERSION || "v2";

function loadPromptTemplate(version: string): string {
  const promptPath = path.join(__dirname, "prompts", version, "spec_gen.txt");
  if (!fs.existsSync(promptPath)) {
    const promptDir = path.join(__dirname, "prompts");
    const available = fs.existsSync(promptDir)
      ? fs
          .readdirSync(promptDir)
          .filter((p) => fs.statSync(path.join(promptDir, p)).isDirectory())
      : [];
    throw new Error(
      `Prompt template not found for version '${version}'. Available versions: ${available.join(", ") || "none"}`
    );
  }
  return fs.readFileSync(promptPath, "utf8");
}

function renderPrompt(template: string, requirement: string): string {
  return template.replace(/\{\{\s*requirement\s*\}\}/g, requirement);
}

function extractCodeBlocks(content: string): [string, string] {
  const mdMatch = content.match(/```markdown\s*\n([\s\S]*?)\n```/);
  const pyMatch = content.match(/```python\s*\n([\s\S]*?)\n```/);
  if (!mdMatch || !pyMatch) {
    throw new Error(
      "Model response did not contain both required markdown and python code blocks."
    );
  }
  return [mdMatch[1].trim(), pyMatch[1].trim()];
}

async function generateArtifacts(
  client: OpenAIClient,
  requirement: string,
  version: string
): Promise<[string, string]> {
  const template = loadPromptTemplate(version);
  const prompt = renderPrompt(template, requirement);

  console.log(`Using prompt version: ${version}`);
  console.log(`Sending ${prompt.length} prompt characters to model.`);

  const response = await client.chatCompletion({
    messages: [{ role: "user", content: prompt }],
    temperature: 0.2,
    max_tokens: 1500,
  });

  const content = response.choices[0].message.content;
  const [spec, scaffold] = extractCodeBlocks(content);
  return [spec, scaffold];
}

function writeArtifacts(spec: string, scaffold: string, outputDir: string): void {
  fs.mkdirSync(outputDir, { recursive: true });
  const specPath = path.join(outputDir, "api_spec.md");
  const scaffoldPath = path.join(outputDir, "scaffold.py");
  fs.writeFileSync(specPath, spec + "\n", "utf8");
  fs.writeFileSync(scaffoldPath, scaffold + "\n", "utf8");
  console.log(`Wrote artifacts to ${outputDir}`);
  console.log(`  - ${path.basename(specPath)}`);
  console.log(`  - ${path.basename(scaffoldPath)}`);
}

function preview(text: string, maxChars = 400): string {
  if (text.length <= maxChars) return text;
  return text.slice(0, maxChars).trimEnd() + "\n...";
}

async function main() {
  const client = new OpenAIClient();

  let spec: string;
  let scaffold: string;
  try {
    [spec, scaffold] = await generateArtifacts(client, REQUIREMENT, PROMPT_VERSION);
  } catch (exc: any) {
    console.error("Failed to generate artifacts:", exc.message);
    process.exit(1);
  }

  const outputDir = path.join(__dirname, "generated");
  try {
    writeArtifacts(spec, scaffold, outputDir);
  } catch (exc: any) {
    console.error("Failed to write artifacts:", exc.message);
    process.exit(1);
  }

  console.log("\n=== Generated API Spec Preview ===");
  console.log(preview(spec));
  console.log("\n=== Generated Scaffold Preview ===");
  console.log(preview(scaffold));
  console.log(`\nArtifacts written to: ${path.resolve(outputDir)}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
