import fs from "fs";
import path from "path";
import readline from "readline";
import { exec } from "child_process";
import { promisify } from "util";
import { OpenAIClient } from "../../../shared/config/openai_client";

const execAsync = promisify(exec);

const ALLOWED_COMMANDS = [
  "systemctl restart",
  "kubectl rollout restart",
  "docker restart",
  "echo",
  "df",
];

function parseLogs(filePath: string): any[] {
  const pattern = /^(\S+)\s+(\w+)\s+(.+)$/;
  const entries: any[] = [];
  for (const line of fs.readFileSync(filePath, "utf8").split("\n")) {
    if (!line) continue;
    const match = line.match(pattern);
    if (match) {
      entries.push({ ts: match[1], level: match[2], message: match[3] });
    } else {
      entries.push({ ts: "", level: "UNKNOWN", message: line });
    }
  }
  return entries;
}

function summarizeErrors(entries: any[]): { error_messages: string[]; counts: Record<string, number> } {
  const errorMessages = entries
    .filter((e) => ["ERROR", "FATAL"].includes(e.level))
    .map((e) => e.message);
  const counts: Record<string, number> = {};
  for (const e of entries) {
    counts[e.level] = (counts[e.level] || 0) + 1;
  }
  return { error_messages: errorMessages, counts };
}

async function diagnose(entries: any[], client: OpenAIClient | null): Promise<any> {
  if (!client) {
    return {
      diagnosis:
        "Database appears unreachable (multiple connection timeouts and 503 errors). " +
        "Disk usage is also elevated.",
      commands: [
        "echo 'Checking database service status...'",
        "systemctl restart postgresql",
        "echo 'Monitoring disk usage...'",
        "df -h",
      ],
      risk: "medium",
    };
  }

  const logText = entries.map((e) => `${e.level}: ${e.message}`).join("\n");
  const prompt = `You are an SRE agent. Diagnose the following logs and respond with JSON in this exact shape:
{
  "diagnosis": "short diagnosis",
  "commands": ["command1", "command2"],
  "risk": "low|medium|high"
}

Logs:
${logText}`;

  const resp = await client.chatCompletion({
    messages: [{ role: "user", content: prompt }],
    temperature: 0.2,
    max_tokens: 200,
    response_format: { type: "json_object" },
  });
  const content = resp.choices[0].message.content;
  try {
    return JSON.parse(content);
  } catch {
    return { diagnosis: content, commands: [], risk: "unknown" };
  }
}

function isCommandAllowed(command: string): boolean {
  return ALLOWED_COMMANDS.some((prefix) => command.trim().startsWith(prefix));
}

async function executeCommands(commands: string[]): Promise<any[]> {
  const results: any[] = [];
  for (const cmd of commands) {
    console.log(`  $ ${cmd}`);
    if (!isCommandAllowed(cmd)) {
      console.log("    -> SKIPPED (not in allowlist)");
      results.push({ command: cmd, status: "skipped", output: "" });
      continue;
    }
    try {
      const { stdout, stderr, error } = (await execAsync(cmd, { timeout: 10000 })) as any;
      const output = (stdout + stderr).trim() || "<no output>";
      const code = error ? error.code : 0;
      console.log(`    -> exit ${code}`);
      results.push({ command: cmd, status: "executed", output });
    } catch (exc: any) {
      console.log(`    -> ERROR: ${exc.message}`);
      results.push({ command: cmd, status: "error", output: String(exc) });
    }
  }
  return results;
}

async function askApproval(): Promise<boolean> {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  const answer = await new Promise<string>((resolve) => {
    rl.question("\nExecute proposed commands? [y/n]: ", resolve);
  });
  rl.close();
  return ["y", "yes"].includes(answer.trim().toLowerCase());
}

async function main() {
  let client: OpenAIClient | null = null;
  try {
    client = new OpenAIClient();
  } catch (exc: any) {
    console.warn("LLM client disabled:", exc.message);
  }

  const logPath = process.argv[2]
    ? path.resolve(process.argv[2])
    : path.resolve("sample_app.log");

  if (!fs.existsSync(logPath)) {
    console.error("Log file not found:", logPath);
    process.exit(1);
  }

  const entries = parseLogs(logPath);
  console.log(`Read ${entries.length} log entries from ${logPath}`);
  const diagnosis = await diagnose(entries, client);

  console.log("\n=== Diagnosis ===");
  console.log(diagnosis.diagnosis);
  console.log(`Risk: ${diagnosis.risk || "unknown"}`);

  const commands = diagnosis.commands || [];
  console.log("\n=== Proposed remediation commands ===");
  for (const cmd of commands) console.log(`  - ${cmd}`);

  if (commands.length === 0) {
    console.log("No remediation commands proposed.");
    return;
  }

  const approved = await askApproval();
  if (!approved) {
    console.log("Execution cancelled by operator.");
    return;
  }

  console.log("\nExecuting commands...");
  const results = await executeCommands(commands);
  console.log("\nExecution summary:");
  console.log(JSON.stringify(results, null, 2));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
