"""Router + Worker + Aggregator multi-agent pattern.

A router decides which specialist agents should handle a user request.
Each worker produces a partial answer. An aggregator combines them.
"""
import json
import logging
import sys
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("multi-agent-patterns")

AGENTS = {
    "coding": {
        "system": (
            "You are a coding assistant. Answer the user's programming question "
            "with concise code and explanation."
        ),
        "fallback": "Use Python functions and add type hints for clarity.",
    },
    "writing": {
        "system": (
            "You are a writing assistant. Improve clarity, grammar, and tone."
        ),
        "fallback": "Use short sentences and active voice.",
    },
    "math": {
        "system": "You are a math assistant. Solve the problem step by step.",
        "fallback": "Break the problem into smaller equations.",
    },
}


class MultiAgentSystem:
    def __init__(self, client: OpenAIClient | None) -> None:
        self.client = client

    def route(self, request: str) -> list[str]:
        if not self.client:
            logger.info("No LLM; routing by keyword fallback")
            lowered = request.lower()
            topics = []
            if any(k in lowered for k in ["code", "python", "function", "error"]):
                topics.append("coding")
            if any(k in lowered for k in ["write", "essay", "grammar", "draft"]):
                topics.append("writing")
            if any(k in lowered for k in ["math", "calculate", "equation", "sum"]):
                topics.append("math")
            if not topics:
                topics = ["writing"]
            return topics

        messages = [
            {
                "role": "system",
                "content": (
                    "You are a router. Given a user request, choose one or more "
                    "specialist topics from: coding, writing, math. Reply with a "
                    "JSON array of strings only, e.g. [\"coding\"]."
                ),
            },
            {"role": "user", "content": request},
        ]
        resp = self.client.chat_completion(
            messages=messages,
            temperature=0.0,
            max_tokens=50,
            response_format={"type": "json_object"},
        )
        content = resp["choices"][0]["message"]["content"]
        try:
            parsed = json.loads(content)
            if isinstance(parsed, list):
                return [t for t in parsed if t in AGENTS]
            if isinstance(parsed, dict):
                return [t for t in parsed.get("topics", []) if t in AGENTS]
        except json.JSONDecodeError:
            pass
        return ["writing"]

    def worker(self, topic: str, request: str) -> dict[str, Any]:
        cfg = AGENTS[topic]
        if not self.client:
            answer = cfg["fallback"]
        else:
            messages = [
                {"role": "system", "content": cfg["system"]},
                {"role": "user", "content": request},
            ]
            resp = self.client.chat_completion(
                messages=messages, temperature=0.3, max_tokens=200
            )
            answer = resp["choices"][0]["message"]["content"].strip()
        return {"topic": topic, "answer": answer}

    def aggregate(self, request: str, responses: list[dict[str, Any]]) -> str:
        if not self.client:
            parts = [f"### {r['topic']}\n{r['answer']}" for r in responses]
            return "\n\n".join(parts)

        combined = "\n\n".join(
            f"### {r['topic']}\n{r['answer']}" for r in responses
        )
        messages = [
            {
                "role": "system",
                "content": (
                    "You are an aggregator. Combine the specialist answers into "
                    "a single coherent response."
                ),
            },
            {
                "role": "user",
                "content": (
                    f"User request: {request}\n\n"
                    f"Specialist answers:\n{combined}\n\nProvide a final answer."
                ),
            },
        ]
        resp = self.client.chat_completion(
            messages=messages, temperature=0.3, max_tokens=300
        )
        return resp["choices"][0]["message"]["content"].strip()

    def run(self, request: str) -> dict[str, Any]:
        topics = self.route(request)
        logger.info("Routed to: %s", topics)
        responses = [self.worker(t, request) for t in topics]
        final = self.aggregate(request, responses)
        return {"topics": topics, "responses": responses, "final_answer": final}


def main() -> None:
    configure_logging()
    client = None
    try:
        client = OpenAIClient()
    except ValueError as exc:
        logger.warning("LLM client disabled: %s", exc)

    system = MultiAgentSystem(client)
    request = "How do I write a Python function that retries a failing operation?"
    print("User request:", request)
    result = system.run(request)
    print("\nRouted to:", result["topics"])
    print("\nSpecialist answers:")
    for r in result["responses"]:
        print(f"  [{r['topic']}] {r['answer'][:200]}...")
    print("\nFinal aggregated answer:")
    print(result["final_answer"])


if __name__ == "__main__":
    main()
