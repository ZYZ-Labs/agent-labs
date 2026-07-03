"""Minimal DAG workflow engine.

Defines tasks with dependencies, executes them in topological order,
retries failed tasks, and collects results.
"""
import json
import logging
import sys
import time
from pathlib import Path
from typing import Any, Callable

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("workflow-orchestration")


class Task:
    def __init__(
        self,
        name: str,
        func: Callable[..., Any],
        deps: list[str] | None = None,
        retries: int = 2,
    ) -> None:
        self.name = name
        self.func = func
        self.deps = deps or []
        self.retries = retries
        self.result: Any = None
        self.error: Exception | None = None


class WorkflowEngine:
    def __init__(self) -> None:
        self.tasks: dict[str, Task] = {}

    def add_task(self, task: Task) -> None:
        self.tasks[task.name] = task

    def _topological_sort(self) -> list[str]:
        in_degree = {name: 0 for name in self.tasks}
        dependents: dict[str, list[str]] = {name: [] for name in self.tasks}
        for name, task in self.tasks.items():
            for dep in task.deps:
                if dep not in self.tasks:
                    raise ValueError(f"Task {name} depends on unknown task {dep}")
                in_degree[name] += 1
                dependents[dep].append(name)

        queue = [name for name, deg in in_degree.items() if deg == 0]
        ordered: list[str] = []
        while queue:
            current = queue.pop(0)
            ordered.append(current)
            for dependent in dependents[current]:
                in_degree[dependent] -= 1
                if in_degree[dependent] == 0:
                    queue.append(dependent)

        if len(ordered) != len(self.tasks):
            raise ValueError("Cycle detected in task dependencies")

        return ordered

    def run(self) -> dict[str, Any]:
        order = self._topological_sort()
        logger.info("Execution order: %s", order)
        for name in order:
            task = self.tasks[name]
            deps_results = {dep: self.tasks[dep].result for dep in task.deps}
            for attempt in range(1, task.retries + 1):
                try:
                    logger.info(
                        "Running task '%s' (attempt %d/%d)", name, attempt, task.retries
                    )
                    task.result = task.func(**deps_results)
                    task.error = None
                    break
                except Exception as exc:
                    task.error = exc
                    logger.warning(
                        "Task '%s' attempt %d failed: %s", name, attempt, exc
                    )
                    if attempt == task.retries:
                        logger.error("Task '%s' exhausted retries", name)
                        raise
                    time.sleep(0.5 * attempt)
            logger.info("Task '%s' completed", name)
        return {name: task.result for name, task in self.tasks.items()}


def fetch_data() -> dict[str, str]:
    return {
        "title": "AI Agent Engineering",
        "content": "Workflow orchestration is essential for reliable agent systems.",
    }


def make_analyze_sentiment(client: OpenAIClient | None) -> Callable[..., str]:
    def _analyze(fetch: dict[str, str]) -> str:
        text = fetch["content"]
        if not client:
            logger.info("No LLM available; using deterministic sentiment fallback")
            return "positive"
        messages = [
            {
                "role": "system",
                "content": (
                    "Classify sentiment as exactly one word: positive, negative, or neutral."
                ),
            },
            {"role": "user", "content": text},
        ]
        resp = client.chat_completion(
            messages=messages, temperature=0.0, max_tokens=10
        )
        return resp["choices"][0]["message"]["content"].strip().lower()

    return _analyze


def make_flaky_quality_check() -> Callable[..., str]:
    """Simulates a flaky validation step that fails the first two attempts."""
    call_count = 0

    def _check(sentiment: str) -> str:
        nonlocal call_count
        call_count += 1
        if call_count < 3:
            raise RuntimeError(
                f"Quality check service unavailable (attempt {call_count})"
            )
        return f"quality_ok ({sentiment})"

    return _check


def make_summarize(client: OpenAIClient | None) -> Callable[..., str]:
    def _summary(fetch: dict[str, str], sentiment: str) -> str:
        if not client:
            logger.info("No LLM available; using deterministic summary fallback")
            return f"Summary: '{fetch['title']}' has {sentiment} sentiment."
        prompt = (
            f"Title: {fetch['title']}\n"
            f"Content: {fetch['content']}\n"
            f"Sentiment: {sentiment}\n"
            "Write a one-sentence summary."
        )
        resp = client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            temperature=0.0,
            max_tokens=60,
        )
        return resp["choices"][0]["message"]["content"].strip()

    return _summary


def main() -> None:
    configure_logging()
    client = None
    try:
        client = OpenAIClient()
    except ValueError as exc:
        logger.warning("LLM client disabled: %s", exc)

    engine = WorkflowEngine()
    engine.add_task(Task(name="fetch", func=fetch_data))
    engine.add_task(
        Task(
            name="sentiment",
            func=make_analyze_sentiment(client),
            deps=["fetch"],
        )
    )
    engine.add_task(
        Task(
            name="quality",
            func=make_flaky_quality_check(),
            deps=["sentiment"],
            retries=5,
        )
    )
    engine.add_task(
        Task(
            name="summary",
            func=make_summarize(client),
            deps=["fetch", "sentiment"],
        )
    )

    print("Starting DAG workflow...")
    results = engine.run()
    print("\nFinal results:")
    print(json.dumps(results, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
