"""Minimal Temporal worker."""
import asyncio
import logging
import sys
from datetime import timedelta
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from temporalio import activity, workflow
from temporalio.client import Client
from temporalio.worker import Worker

from openai_client import configure_logging

logger = logging.getLogger("temporal-worker")

TASK_QUEUE = "agent-labs-greeting-queue"


@activity.defn
async def compose_greeting(name: str) -> str:
    logger.info("Activity called with name=%s", name)
    return f"Hello, {name} from Temporal!"


@workflow.defn
class GreetingWorkflow:
    @workflow.run
    async def run(self, name: str) -> str:
        logger.info("Workflow started for name=%s", name)
        return await workflow.execute_activity(
            compose_greeting,
            name,
            start_to_close_timeout=timedelta(seconds=10),
        )


async def main() -> None:
    configure_logging()
    try:
        client = await Client.connect("localhost:7233")
    except Exception as exc:
        logger.error("Could not connect to Temporal server at localhost:7233: %s", exc)
        print("\nStart Temporal first with:")
        print("  docker compose up -d")
        return

    worker = Worker(
        client,
        task_queue=TASK_QUEUE,
        workflows=[GreetingWorkflow],
        activities=[compose_greeting],
    )
    logger.info("Starting worker on task queue '%s'...", TASK_QUEUE)
    await worker.run()


if __name__ == "__main__":
    asyncio.run(main())
