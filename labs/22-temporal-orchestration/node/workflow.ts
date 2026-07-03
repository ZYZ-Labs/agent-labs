import { proxyActivities } from "@temporalio/workflow";

const { composeGreeting } = proxyActivities({
  startToCloseTimeout: "10 seconds",
});

export async function greetingWorkflow(name: string): Promise<string> {
  return await composeGreeting(name);
}
