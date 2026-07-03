import fs from "fs";
import os from "os";
import path from "path";
import { spawnSync } from "child_process";
import { OpenAIClient } from "../../../shared/config/openai_client";

const DESCRIPTION =
  "A blog post resource with title (required string), body (required string), " +
  "and published (boolean, default false). Provide CRUD endpoints for listing, " +
  "creating, reading, updating, and deleting posts. Store posts in memory.";

const FILES: Record<string, RegExp> = {
  "models.py": /```python\s*models\.py\s*\n([\s\S]*?)\n```/,
  "routes.py": /```python\s*routes\.py\s*\n([\s\S]*?)\n```/,
  "test_crud.py": /```python\s*test_crud\.py\s*\n([\s\S]*?)\n```/,
};

function loadPrompt(description: string): string {
  const promptPath = path.join(__dirname, "prompts", "crud_gen.txt");
  const template = fs.readFileSync(promptPath, "utf8");
  return template.replace(/\{\{\s*description\s*\}\}/g, description);
}

function extractFile(content: string, pattern: RegExp): string {
  const match = content.match(pattern);
  if (!match) throw new Error("Required code block not found in model response.");
  return match[1].trim();
}

function validatePython(source: string, filename: string): void {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "lab15-"));
  const tmpFile = path.join(tmpDir, filename);
  fs.writeFileSync(tmpFile, source, "utf8");
  try {
    const result = spawnSync("python", ["-m", "py_compile", tmpFile], {
      encoding: "utf8",
    });
    if (result.status !== 0) {
      throw new Error(result.stderr || `Python compile error in ${filename}`);
    }
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }

  if (filename === "routes.py" && !source.includes("APIRouter")) {
    console.warn(`${filename} does not appear to define an APIRouter.`);
  }
}

async function generateModule(client: OpenAIClient, description: string): Promise<Record<string, string>> {
  const prompt = loadPrompt(description);
  console.log(`Sending backend generation prompt (${prompt.length} chars).`);

  const response = await client.chatCompletion({
    messages: [{ role: "user", content: prompt }],
    temperature: 0.2,
    max_tokens: 2000,
  });

  const content = response.choices[0].message.content;
  const generated: Record<string, string> = {};
  for (const [filename, pattern] of Object.entries(FILES)) {
    generated[filename] = extractFile(content, pattern);
  }
  return generated;
}

function writeModule(files: Record<string, string>, outputDir: string): void {
  fs.mkdirSync(outputDir, { recursive: true });
  for (const [filename, source] of Object.entries(files)) {
    fs.writeFileSync(path.join(outputDir, filename), source + "\n", "utf8");
    console.log(`Wrote ${path.join(outputDir, filename)}`);
  }
}

async function main() {
  const client = new OpenAIClient();

  let files: Record<string, string>;
  try {
    files = await generateModule(client, DESCRIPTION);
  } catch (exc: any) {
    console.error("Generation failed:", exc.message);
    process.exit(1);
  }

  let allValid = true;
  for (const [filename, source] of Object.entries(files)) {
    try {
      validatePython(source, filename);
      console.log(`[OK] ${filename} parses and compiles.`);
    } catch (exc: any) {
      allValid = false;
      console.log(`[FAIL] ${exc.message}`);
    }
  }

  const outputDir = path.join(__dirname, "generated");
  try {
    writeModule(files, outputDir);
  } catch (exc: any) {
    console.error("Failed to write generated module:", exc.message);
    process.exit(1);
  }

  if (!allValid) {
    console.error("One or more generated files failed validation.");
    process.exit(1);
  }

  console.log(`\nGenerated module written to: ${path.resolve(outputDir)}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
