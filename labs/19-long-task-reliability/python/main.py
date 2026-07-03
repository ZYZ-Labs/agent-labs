"""Long-task reliability with checkpoints, idempotency keys, retries, and timeouts.

Simulates a multi-step long task that can fail and resume from the last checkpoint.
"""
import json
import logging
import sys
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, TimeoutError as FutureTimeoutError
from pathlib import Path
from typing import Any, Callable

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("long-task-reliability")

CHECKPOINT_DIR = Path(__file__).with_suffix("").parent / ".checkpoints"
CHECKPOINT_DIR.mkdir(exist_ok=True)


class CheckpointStore:
    def __init__(self, key: str) -> None:
        self.path = CHECKPOINT_DIR / f"{key}.json"

    def load(self) -> dict[str, Any]:
        if self.path.exists():
            with open(self.path, "r", encoding="utf-8") as f:
                return json.load(f)
        return {}

    def save(self, state: dict[str, Any]) -> None:
        tmp = self.path.with_suffix(".tmp")
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(state, f, indent=2)
        tmp.replace(self.path)


def exponential_backoff(
    attempt: int, base_delay: float = 1.0, max_delay: float = 30.0
) -> float:
    delay = min(base_delay * (2 ** (attempt - 1)), max_delay)
    jitter = delay * 0.1 * (1 if attempt % 2 == 0 else -1)
    return delay + jitter


def run_with_timeout(func: Callable[..., Any], timeout: float, *args: Any, **kwargs: Any) -> Any:
    with ThreadPoolExecutor(max_workers=1) as executor:
        future = executor.submit(func, *args, **kwargs)
        try:
            return future.result(timeout=timeout)
        except FutureTimeoutError as exc:
            raise TimeoutError(f"Step timed out after {timeout}s") from exc


class LongTask:
    def __init__(self, input_data: str, client: OpenAIClient | None) -> None:
        self.input_data = input_data
        self.client = client
        self.idempotency_key = self._make_key(input_data)
        self.store = CheckpointStore(self.idempotency_key)
        self.state = self.store.load()
        self._flaky_attempts = 0

    def _make_key(self, input_data: str) -> str:
        # Stable key derived from input; in production use a business key.
        key = uuid.uuid5(uuid.NAMESPACE_DNS, input_data)
        return f"task-{key}"

    def _is_complete(self) -> bool:
        return (
            self.state.get("status") == "completed"
            and self.state.get("idempotency_key") == self.idempotency_key
        )

    def _run_step(
        self,
        name: str,
        func: Callable[..., Any],
        timeout: float = 5.0,
        max_retries: int = 3,
    ) -> Any:
        if self.state.get("completed_steps", {}).get(name):
            logger.info("Step '%s' already completed; skipping", name)
            return self.state["results"][name]

        logger.info("Executing step '%s'", name)
        last_error: Exception | None = None
        for attempt in range(1, max_retries + 1):
            try:
                result = run_with_timeout(func, timeout)
                completed = self.state.setdefault("completed_steps", {})
                completed[name] = True
                results = self.state.setdefault("results", {})
                results[name] = result
                self.state["last_step"] = name
                self.store.save(self.state)
                logger.info("Step '%s' succeeded", name)
                return result
            except Exception as exc:
                last_error = exc
                logger.warning("Step '%s' attempt %d failed: %s", name, attempt, exc)
                if attempt < max_retries:
                    delay = exponential_backoff(attempt)
                    logger.info("Retrying step '%s' in %.2fs", name, delay)
                    time.sleep(delay)
        logger.error("Step '%s' exhausted retries", name)
        raise last_error

    def step_fetch_data(self) -> str:
        logger.info("Fetching data for: %s", self.input_data)
        if self.client:
            messages = [
                {
                    "role": "user",
                    "content": f"Summarize '{self.input_data}' in one sentence.",
                }
            ]
            resp = self.client.chat_completion(messages=messages, max_tokens=50)
            return resp["choices"][0]["message"]["content"].strip()
        return f"Mock summary for '{self.input_data}'."

    def step_process_data(self, fetched: str) -> str:
        # Simulate flaky processing.
        self._flaky_attempts += 1
        if self._flaky_attempts < 3:
            raise RuntimeError(
                f"Processing service busy (attempt {self._flaky_attempts})"
            )
        return f"processed({fetched})"

    def step_notify(self, processed: str) -> str:
        return f"notification_sent({processed})"

    def run(self) -> dict[str, Any]:
        if self._is_complete():
            logger.info(
                "Task already completed for key %s; returning cached result",
                self.idempotency_key,
            )
            return {"status": "completed", "results": self.state["results"]}

        self.state.setdefault("idempotency_key", self.idempotency_key)
        self.state.setdefault("status", "running")
        self.state.setdefault("completed_steps", {})
        self.state.setdefault("results", {})

        fetched = self._run_step("fetch", self.step_fetch_data)
        processed = self._run_step(
            "process", lambda: self.step_process_data(fetched)
        )
        notified = self._run_step("notify", lambda: self.step_notify(processed))

        self.state["status"] = "completed"
        self.store.save(self.state)
        logger.info("Task completed successfully")
        return {"status": "completed", "results": self.state["results"]}


def main() -> None:
    configure_logging()
    client = None
    try:
        client = OpenAIClient()
    except ValueError as exc:
        logger.warning("LLM client disabled: %s", exc)

    input_data = "reliable agent engineering"
    task = LongTask(input_data, client)

    print("Starting long-running task...")
    print("Idempotency key:", task.idempotency_key)
    result = task.run()
    print("\nFinal result:")
    print(json.dumps(result, indent=2, ensure_ascii=False))

    # Demonstrate idempotency by re-running.
    print("\nRe-running with the same idempotency key...")
    task2 = LongTask(input_data, client)
    result2 = task2.run()
    print(json.dumps(result2, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
