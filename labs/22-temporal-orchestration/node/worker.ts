export const TASK_QUEUE = "agent-labs-greeting-queue";

async function main() {
  console.log(`Worker placeholder for task queue: ${TASK_QUEUE}`);
  console.log("In production, use @temporalio/worker Worker.create with a workflow bundle.");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
