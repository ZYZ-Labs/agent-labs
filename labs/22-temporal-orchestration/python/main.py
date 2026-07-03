"""Minimal Temporal client."""
import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from temporalio.client import Client
from temporalio.exceptions import WorkflowAlreadyStartedError

from openai_client import configure_logging
from worker import GreetingWorkflow, TASK_QUEUE

logger = logging.getLogger("temporal-client")


async def main() -> None:
    configure_logging()
    try:
        client = await Client.connect("localhost:7233")
    except Exception as exc:
        logger.error("Could not connect to Temporal server at localhost:7233: %s", exc)
        print("\nStart Temporal first with:")
        print("  docker compose up -d")
        print("Then start the worker:")
        print("  python worker.py")
        return

    workflow_id = "greeting-workflow-001"
    logger.info("Starting workflow %s...", workflow_id)
    try:
        result = await client.execute_workflow(
            GreetingWorkflow.run,
            "Agent Engineer",
            id=workflow_id,
            task_queue=TASK_QUEUE,
        )
    except WorkflowAlreadyStartedError:
        handle = client.get_workflow_handle(workflow_id)
        result = await handle.result()
    print("Workflow result:", result)


if __name__ == "__main__":
    asyncio.run(main())
