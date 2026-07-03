"""Shared OpenAI-compatible client wrapper for Python labs.

Reads configuration from environment variables and provides a simple
chat.completions.create-style helper with structured logging and retries.
"""

import json
import logging
import os
from typing import Any

import httpx

logger = logging.getLogger("agent-labs")

DEFAULT_TIMEOUT = 60.0
DEFAULT_MAX_RETRIES = 3


def load_env(path: str | None = None) -> None:
    """Load .env file if python-dotenv is available."""
    try:
        from dotenv import load_dotenv

        load_dotenv(path or os.path.join(os.getcwd(), ".env"))
    except ImportError:
        pass


def get_config() -> dict[str, str]:
    load_env()
    return {
        "api_key": os.getenv("OPENAI_API_KEY", ""),
        "base_url": os.getenv("OPENAI_BASE_URL", "https://api.openai.com/v1").rstrip("/"),
        "model": os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
        "log_level": os.getenv("LOG_LEVEL", "INFO"),
    }


def configure_logging(level: str | None = None) -> None:
    cfg = get_config()
    logging.basicConfig(
        level=getattr(logging, (level or cfg["log_level"]).upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )


class OpenAIClient:
    def __init__(
        self,
        api_key: str | None = None,
        base_url: str | None = None,
        model: str | None = None,
        timeout: float = DEFAULT_TIMEOUT,
        max_retries: int = DEFAULT_MAX_RETRIES,
    ) -> None:
        cfg = get_config()
        self.api_key = api_key or cfg["api_key"]
        self.base_url = base_url or cfg["base_url"]
        self.model = model or cfg["model"]
        self.timeout = timeout
        self.max_retries = max_retries
        if not self.api_key and not self.base_url.startswith("http://localhost"):
            raise ValueError("OPENAI_API_KEY is required for non-local endpoints")

    def chat_completion(
        self,
        messages: list[dict[str, str]],
        model: str | None = None,
        temperature: float | None = None,
        max_tokens: int | None = None,
        top_p: float | None = None,
        frequency_penalty: float | None = None,
        presence_penalty: float | None = None,
        stop: list[str] | str | None = None,
        seed: int | None = None,
        response_format: dict[str, str] | None = None,
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict[str, Any] | None = None,
        stream: bool = False,
        extra_body: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        url = f"{self.base_url}/chat/completions"
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
        }
        payload: dict[str, Any] = {
            "model": model or self.model,
            "messages": messages,
            "stream": stream,
        }
        if temperature is not None:
            payload["temperature"] = temperature
        if max_tokens is not None:
            payload["max_tokens"] = max_tokens
        if top_p is not None:
            payload["top_p"] = top_p
        if frequency_penalty is not None:
            payload["frequency_penalty"] = frequency_penalty
        if presence_penalty is not None:
            payload["presence_penalty"] = presence_penalty
        if stop is not None:
            payload["stop"] = stop
        if seed is not None:
            payload["seed"] = seed
        if response_format is not None:
            payload["response_format"] = response_format
        if tools is not None:
            payload["tools"] = tools
        if tool_choice is not None:
            payload["tool_choice"] = tool_choice
        if extra_body:
            payload.update(extra_body)

        logger.debug("POST %s with payload: %s", url, json.dumps(payload, ensure_ascii=False))

        for attempt in range(1, self.max_retries + 1):
            try:
                with httpx.Client(timeout=self.timeout) as client:
                    response = client.post(url, headers=headers, json=payload)
                    response.raise_for_status()
                    return response.json()
            except (httpx.HTTPStatusError, httpx.NetworkError, httpx.TimeoutException) as exc:
                logger.warning("Request attempt %d failed: %s", attempt, exc)
                if attempt == self.max_retries:
                    raise
        raise RuntimeError("Unreachable")

    def extract_message(self, response: dict[str, Any]) -> dict[str, Any]:
        return response["choices"][0]["message"]


if __name__ == "__main__":
    configure_logging()
    client = OpenAIClient()
    resp = client.chat_completion(
        messages=[{"role": "user", "content": "Say hello in one word."}],
        max_tokens=10,
    )
    print(resp["choices"][0]["message"]["content"])
