import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


def run_case(client: OpenAIClient, name: str, params: dict, messages: list) -> None:
    print(f"\n=== {name} ===")
    try:
        response = client.chat_completion(messages=messages, **params)
    except Exception as exc:
        print(f"Error: {exc}")
        return

    for idx, choice in enumerate(response["choices"]):
        prefix = f"  Choice {idx}:" if len(response["choices"]) > 1 else "  Output:"
        print(f"{prefix} {choice['message']['content'].strip()}")
    print(f"  Finish reason: {response['choices'][0]['finish_reason']}")
    print(f"  Usage: {response.get('usage', {})}")


def main():
    configure_logging()
    client = OpenAIClient()

    messages = [
        {
            "role": "system",
            "content": "You are a helpful coding assistant. Be concise.",
        },
        {
            "role": "user",
            "content": (
                "List three benefits of using state machines to model agent workflows. "
                "Answer in at most two sentences."
            ),
        },
    ]

    cases = [
        ("Default", {"max_tokens": 120}),
        ("High temperature (creative)", {"temperature": 1.2, "max_tokens": 120}),
        ("Low temperature (deterministic)", {"temperature": 0.0, "max_tokens": 120}),
        ("top_p nucleus sampling", {"top_p": 0.3, "max_tokens": 120}),
        ("Frequency penalty", {"frequency_penalty": 1.0, "max_tokens": 120}),
        ("Presence penalty", {"presence_penalty": 1.0, "max_tokens": 120}),
        ("Stop sequence", {"stop": ["."], "max_tokens": 120}),
        ("Seed for reproducibility", {"seed": 42, "temperature": 0.0, "max_tokens": 120}),
        (
            "JSON response format",
            {
                "response_format": {"type": "json_object"},
                "messages": messages
                + [
                    {
                        "role": "user",
                        "content": "Return the answer as JSON with keys: summary, benefits.",
                    }
                ],
            },
        ),
        ("Multiple choices n=3", {"n": 3, "temperature": 1.0, "max_tokens": 60}),
    ]

    for name, params in cases:
        # Use custom messages if provided, otherwise default
        msgs = params.pop("messages", messages)
        run_case(client, name, params, msgs)


if __name__ == "__main__":
    main()
