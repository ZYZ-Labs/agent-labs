import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


def main():
    configure_logging()
    client = OpenAIClient()

    question = "Say hello in one sentence."
    print(f"User: {question}")

    response = client.chat_completion(
        messages=[{"role": "user", "content": question}],
        max_tokens=50,
        temperature=0.0,
    )
    answer = response["choices"][0]["message"].get("content", "")
    print(f"Assistant: {answer}")


if __name__ == "__main__":
    main()
