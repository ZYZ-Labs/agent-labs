import fs from "fs";
import path from "path";
import readline from "readline";

const STATE_FILE = path.join(__dirname, "workflow_state.json");

class DestructiveAction {
  constructor(public target: string, public action: string) {}

  describe(): string {
    return `${this.action} ${this.target}`;
  }
}

class HumanInTheLoopWorkflow {
  private state: any;

  constructor(private stateFile: string) {
    this.state = this.load();
  }

  private load(): any {
    if (fs.existsSync(this.stateFile)) {
      return JSON.parse(fs.readFileSync(this.stateFile, "utf8"));
    }
    return {};
  }

  private save(): void {
    const tmp = `${this.stateFile}.tmp`;
    fs.writeFileSync(tmp, JSON.stringify(this.state, null, 2), "utf8");
    fs.renameSync(tmp, this.stateFile);
  }

  reset(): void {
    if (fs.existsSync(this.stateFile)) fs.unlinkSync(this.stateFile);
    this.state = {};
  }

  requestApproval(action: DestructiveAction): void {
    if (["approved", "executed", "rejected"].includes(this.state.stage)) return;
    console.log(`\nDestructive action requested: ${action.describe()}`);
    console.log("This action cannot be undone. Please review carefully.");
    this.state = {
      stage: "awaiting_approval",
      target: action.target,
      action: action.action,
      approved: null,
    };
    this.save();
  }

  async collectDecision(): Promise<boolean | null> {
    if (this.state.stage !== "awaiting_approval") {
      return this.state.approved;
    }
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
    const ask = (q: string): Promise<string> => new Promise((resolve) => rl.question(q, resolve));

    while (true) {
      const answer = (await ask("Approve? [y/n]: ")).trim().toLowerCase();
      if (answer === "y" || answer === "yes") {
        this.state.approved = true;
        this.state.stage = "approved";
        this.save();
        rl.close();
        return true;
      }
      if (answer === "n" || answer === "no") {
        this.state.approved = false;
        this.state.stage = "rejected";
        this.save();
        rl.close();
        return false;
      }
      console.log("Please answer 'y' or 'n'.");
    }
  }

  execute(action: DestructiveAction): void {
    if (this.state.stage === "executed") {
      console.log("Action already executed.");
      return;
    }
    if (!this.state.approved) {
      console.log("Action not approved; will not execute.");
      return;
    }
    console.log(`\nExecuting: ${action.describe()}`);
    console.log(`  -> ${action.target} has been ${action.action}d.`);
    this.state.stage = "executed";
    this.state.executed_at = new Date().toISOString();
    this.save();
  }

  async run(action: DestructiveAction): Promise<void> {
    this.requestApproval(action);
    const approved = await this.collectDecision();
    if (approved) {
      this.execute(action);
    } else {
      console.log("\nAction rejected by human operator.");
    }
  }
}

async function main() {
  const reset = process.argv.includes("--reset");
  const workflow = new HumanInTheLoopWorkflow(STATE_FILE);
  if (reset) {
    workflow.reset();
    console.log("Workflow state reset.");
    return;
  }

  const action = new DestructiveAction("user_account_42", "delete");
  await workflow.run(action);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
