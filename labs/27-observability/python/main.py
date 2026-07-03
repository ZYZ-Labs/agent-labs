import json
import logging
import sys
import time
import uuid
from contextlib import contextmanager
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

# Structured JSON logger.
logger = logging.getLogger("agent-labs.observability")


class StructuredLogFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        payload = {
            "timestamp": self.formatTime(record),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
        }
        if hasattr(record, "trace_id"):
            payload["trace_id"] = record.trace_id
        if hasattr(record, "span_name"):
            payload["span_name"] = record.span_name
        if hasattr(record, "span_attrs"):
            payload.update(record.span_attrs)
        return json.dumps(payload, ensure_ascii=False)


def setup_logging() -> None:
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(StructuredLogFormatter())
    root = logging.getLogger("agent-labs")
    root.handlers = []
    root.addHandler(handler)
    root.setLevel(logging.INFO)


@contextmanager
def span(name: str, trace_id: str, attrs: dict[str, Any] | None = None):
    start = time.perf_counter()
    extra = {"trace_id": trace_id, "span_name": name, "span_attrs": {"event": "span.start", **(attrs or {})}}
    logger.info("span.start", extra=extra)
    try:
        yield
    finally:
        duration_ms = (time.perf_counter() - start) * 1000
        extra["span_attrs"]["event"] = "span.end"
        extra["span_attrs"]["duration_ms"] = round(duration_ms, 2)
        logger.info("span.end", extra=extra)


def log_usage(trace_id: str, response: dict[str, Any]) -> None:
    usage = response.get("usage") or {}
    extra = {
        "trace_id": trace_id,
        "span_name": "usage",
        "span_attrs": {
            "event": "tokens.usage",
            "prompt_tokens": usage.get("prompt_tokens"),
            "completion_tokens": usage.get("completion_tokens"),
            "total_tokens": usage.get("total_tokens"),
        },
    }
    logger.info("tokens.usage", extra=extra)


def run_observable_agent(client: OpenAIClient, user_message: str) -> str:
    trace_id = str(uuid.uuid4())

    with span("agent.run", trace_id, {"input_length": len(user_message)}):
        with span("llm.call", trace_id):
            response = client.chat_completion(
                messages=[{"role": "user", "content": user_message}],
                max_tokens=200,
                temperature=0.0,
            )
            log_usage(trace_id, response)

        message = response["choices"][0]["message"]
        content = message.get("content") or ""

        # Simulate a tool call if the model returned one.
        tool_calls = message.get("tool_calls") or []
        for tc in tool_calls:
            name = tc["function"]["name"]
            with span("tool.execute", trace_id, {"tool_name": name}):
                # In a real agent, execute the tool here.
                pass

    extra = {"trace_id": trace_id, "span_name": "agent.run", "span_attrs": {"event": "agent.response", "response": content}}
    logger.info("agent.response", extra=extra)
    return content


def main():
    configure_logging()
    setup_logging()
    client = OpenAIClient()

    question = "Explain observability in one sentence."
    print(f"User: {question}")
    answer = run_observable_agent(client, question)
    print(f"Assistant: {answer}")


if __name__ == "__main__":
    main()
