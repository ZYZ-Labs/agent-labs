import { Connection } from "@temporalio/client";

export const TASK_QUEUE = "agent-labs-greeting-queue";

async function main() {
  try {
    await Connection.connect({ address: "localhost:7233" });
    console.log("Connected to Temporal server at localhost:7233");
    console.log(`Task queue: ${TASK_QUEUE}`);
    console.log("\nNext steps:");
    console.log("  npm run worker");
  } catch (exc: any) {
    console.error("Could not connect to Temporal server at localhost:7233:", exc.message);
    console.log("\nStart Temporal first with:");
    console.log("  docker compose up -d");
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
