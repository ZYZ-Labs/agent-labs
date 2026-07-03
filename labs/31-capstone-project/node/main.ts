import fs from "node:fs";
import path from "node:path";
import { OpenAIClient } from "../../../shared/config/openai_client";

const OUTPUT_DIR = path.join(__dirname, "..", "output");

const DEFAULT_REQUIREMENTS = `Build a simple REST API for a task management service.
Users should be able to create, list, update, and delete tasks.
Each task has a title, description, status (todo/in_progress/done), and due date.
Store data in memory; no database is required.
Add basic input validation.`;

function sanitizeFilename(text: string) {
  return text.replace(/[^a-zA-Z0-9_\-]/g, "_").slice(0, 50);
}

function stripMarkdownFences(text: string) {
  text = text.trim();
  if (text.startsWith("```")) {
    const lines = text.split("\n");
    if (lines[0].startsWith("```")) lines.shift();
    if (lines.length && lines[lines.length - 1].startsWith("```")) lines.pop();
    text = lines.join("\n");
  }
  return text.trim();
}

async function generateDesignDoc(client: OpenAIClient, requirements: string) {
  const prompt = `You are a senior backend engineer. Write a concise design document (Markdown) for the following requirements.\n\nRequirements:\n${requirements}\n\nInclude:\n- Overview\n- Endpoints (method, path, description)\n- Data model\n- Assumptions and constraints\n\nRespond with Markdown only.`;
  const response = await client.chatCompletion({ messages: [{ role: "user", content: prompt }], max_tokens: 1500, temperature: 0.2 });
  return client.extractMessage(response).content || "";
}

async function generateAPICode(client: OpenAIClient, requirements: string, designDoc: string) {
  const prompt = `You are a senior backend engineer. Implement a runnable Express.js application in TypeScript for the requirements below.\n\nRequirements:\n${requirements}\n\nDesign document:\n${designDoc}\n\nGuidelines:\n- Use Express and TypeScript.\n- Store data in memory.\n- Include input validation.\n- Do not include instructions or explanations outside the code.\n- Output a single TypeScript file named api.ts.\n\nRespond with the full TypeScript source code only (no Markdown fences).`;
  const response = await client.chatCompletion({ messages: [{ role: "user", content: prompt }], max_tokens: 2000, temperature: 0.2 });
  return stripMarkdownFences(client.extractMessage(response).content || "");
}

async function generateTests(client: OpenAIClient, apiCode: string) {
  const prompt = `You are a QA engineer. Write Jest/Supertest tests for the following Express application.\n\nApplication code:\n${apiCode}\n\nGuidelines:\n- Use supertest.\n- Cover create, list, update, and delete endpoints.\n- Include at least one validation failure test.\n- Output a single TypeScript test file named api.test.ts.\n\nRespond with the full TypeScript test source code only (no Markdown fences).`;
  const response = await client.chatCompletion({ messages: [{ role: "user", content: prompt }], max_tokens: 2000, temperature: 0.2 });
  return stripMarkdownFences(client.extractMessage(response).content || "");
}

async function generateReviewReport(client: OpenAIClient, requirements: string, designDoc: string, apiCode: string, tests: string) {
  const prompt = `You are a staff engineer reviewing the following backend artifact bundle.\n\nRequirements:\n${requirements}\n\nDesign Document:\n${designDoc}\n\nAPI Code:\n${apiCode}\n\nTests:\n${tests}\n\nProduce a review report (Markdown) with:\n- Summary\n- What was done well\n- Risks and concerns\n- Actionable recommendations\n- Pass/needs-work verdict\n\nRespond with Markdown only.`;
  const response = await client.chatCompletion({ messages: [{ role: "user", content: prompt }], max_tokens: 1500, temperature: 0.2 });
  return client.extractMessage(response).content || "";
}

function writeArtifact(name: string, content: string) {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  const filePath = path.join(OUTPUT_DIR, name);
  fs.writeFileSync(filePath, content, "utf8");
  return filePath;
}

async function main() {
  const client = new OpenAIClient();
  const requirements = process.env.REQUIREMENTS || DEFAULT_REQUIREMENTS;
  console.log("Requirements:");
  console.log(requirements);
  console.log();

  console.log("Generating design doc...");
  const designDoc = await generateDesignDoc(client, requirements);
  console.log(`  Wrote ${writeArtifact("design_doc.md", designDoc)}`);

  console.log("Generating API code...");
  const apiCode = await generateAPICode(client, requirements, designDoc);
  console.log(`  Wrote ${writeArtifact("api.ts", apiCode)}`);

  console.log("Generating tests...");
  const tests = await generateTests(client, apiCode);
  console.log(`  Wrote ${writeArtifact("api.test.ts", tests)}`);

  console.log("Generating review report...");
  const review = await generateReviewReport(client, requirements, designDoc, apiCode, tests);
  console.log(`  Wrote ${writeArtifact("review_report.md", review)}`);

  console.log("\nCapstone artifacts generated successfully.");
}

main().catch(console.error);
