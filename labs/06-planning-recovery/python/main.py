import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


SCHEMA = {
    "type": "object",
    "properties": {
        "service_name": {"type": "string"},
        "port": {"type": "integer", "minimum": 1024, "maximum": 65535},
        "replicas": {"type": "integer", "minimum": 1, "maximum": 10},
        "env": {"type": "array", "items": {"type": "string"}},
    },
    "required": ["service_name", "port", "replicas"],
}


def validate_config(config: dict) -> str | None:
    if not isinstance(config, dict):
        return "Config must be a JSON object."
    for key in ["service_name", "port", "replicas"]:
        if key not in config:
            return f"Missing required field: {key}"
    if not (1024 <= config["port"] <= 65535):
        return "port must be between 1024 and 65535."
    if not (1 <= config["replicas"] <= 10):
        return "replicas must be between 1 and 10."
    return None


def generate_with_recovery(client: OpenAIClient, request: str, max_retries: int = 3) -> dict:
    messages = [
        {
            "role": "system",
            "content": (
                "You are a configuration generator. "
                f"Return only valid JSON matching this schema: {json.dumps(SCHEMA)}. "
                "No markdown, no explanation."
            ),
        },
        {"role": "user", "content": request},
    ]

    for attempt in range(1, max_retries + 1):
        print(f"\n--- Attempt {attempt} ---")
        response = client.chat_completion(messages=messages, temperature=0.2, max_tokens=300)
        raw = response["choices"][0]["message"]["content"]
        print("Raw output:", raw)

        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError as exc:
            error = f"Invalid JSON: {exc}"
            print(error)
            messages.append({"role": "assistant", "content": raw})
            messages.append({"role": "user", "content": f"That was not valid JSON. {error} Please retry."})
            continue

        error = validate_config(parsed)
        if error is None:
            return parsed

        print("Validation error:", error)
        messages.append({"role": "assistant", "content": raw})
        messages.append(
            {"role": "user", "content": f"Validation failed: {error}. Fix the JSON and retry."}
        )

    raise RuntimeError("Failed to generate valid config after max retries.")


def main():
    configure_logging()
    client = OpenAIClient()
    request = "Create a config for a payment-api service on port 8080 with 3 replicas and env vars LOG_LEVEL=info,DB_URL=postgres."
    config = generate_with_recovery(client, request)
    print("\nFinal valid config:")
    print(json.dumps(config, indent=2))


if __name__ == "__main__":
    main()
