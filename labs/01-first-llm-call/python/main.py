import json
import sys
from pathlib import Path

# Add shared config to path
sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


def main():
    configure_logging()
    try:
        client = OpenAIClient()
    except ValueError as exc:
        print(f"Configuration error: {exc}", file=sys.stderr)
        sys.exit(1)

    messages = [
        {"role": "system", "content": "You are a concise assistant."},
        {"role": "user", "content": "Explain what an AI agent is in one sentence."},
    ]

    try:
        response = client.chat_completion(messages=messages, max_tokens=80)
    except Exception as exc:
        print(f"Request failed: {exc}", file=sys.stderr)
        sys.exit(1)

    message = client.extract_message(response)
    print("Assistant:", message["content"])
    print("Model:", response.get("model"))
    print("Finish reason:", response["choices"][0].get("finish_reason"))
    print("Usage:", json.dumps(response.get("usage", {}), indent=2))


if __name__ == "__main__":
    main()
