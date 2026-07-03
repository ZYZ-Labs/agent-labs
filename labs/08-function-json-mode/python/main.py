import json
import sys
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


EVENT_SCHEMA = {
    "type": "object",
    "properties": {
        "name": {"type": "string"},
        "date": {"type": "string"},
        "location": {"type": "string"},
        "participants": {"type": "array", "items": {"type": "string"}},
    },
    "required": ["name", "date"],
}

USER_PROMPT = """
Extract the event details from this message as JSON:

"Join us for the AI Engineering Meetup on 2025-09-15 at the Shenzhen Hub.
Attendees: Alice, Bob, and Carol."
""".strip()


def validate_event(data: Any) -> dict[str, Any]:
    """Lightweight schema check; returns a dict of problems or the parsed data."""
    if not isinstance(data, dict):
        raise ValueError("Parsed JSON is not an object")
    missing = [f for f in EVENT_SCHEMA["required"] if f not in data]
    if missing:
        raise ValueError(f"Missing required fields: {missing}")
    for key, value in data.items():
        spec = EVENT_SCHEMA["properties"].get(key)
        if spec and spec.get("type") == "array" and not isinstance(value, list):
            raise ValueError(f"Field '{key}' should be an array")
    return data


def extract_with_json_mode(client: OpenAIClient) -> dict[str, Any]:
    """Use response_format to force a JSON object."""
    response = client.chat_completion(
        messages=[
            {
                "role": "system",
                "content": (
                    "You are a helpful parser. "
                    "Return ONLY a JSON object with keys: name, date, location, participants."
                ),
            },
            {"role": "user", "content": USER_PROMPT},
        ],
        response_format={"type": "json_object"},
        temperature=0.0,
        max_tokens=300,
    )
    raw = response["choices"][0]["message"]["content"]
    print("\n[JSON mode] raw output:")
    print(raw)
    return validate_event(json.loads(raw))


def extract_with_function_call(client: OpenAIClient) -> dict[str, Any]:
    """Use a single tool to request structured arguments."""
    tool = {
        "type": "function",
        "function": {
            "name": "extract_event",
            "description": "Extract event details from user text.",
            "parameters": EVENT_SCHEMA,
        },
    }
    response = client.chat_completion(
        messages=[
            {"role": "system", "content": "Use the extract_event tool."},
            {"role": "user", "content": USER_PROMPT},
        ],
        tools=[tool],
        tool_choice={"type": "function", "function": {"name": "extract_event"}},
        temperature=0.0,
        max_tokens=300,
    )
    message = response["choices"][0]["message"]
    tool_calls = message.get("tool_calls", [])
    if not tool_calls:
        raise ValueError("Model did not call the extract_event tool")
    raw = tool_calls[0]["function"]["arguments"]
    print("\n[Function call] raw arguments:")
    print(raw)
    return validate_event(json.loads(raw))


def run_mode(label: str, extractor, client: OpenAIClient) -> dict[str, Any]:
    print(f"\n{'=' * 40}\nMode: {label}\n{'=' * 40}")
    try:
        result = extractor(client)
        print(f"\n[{label}] parsed event:")
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return {"ok": True, "result": result}
    except (json.JSONDecodeError, ValueError, KeyError) as exc:
        print(f"\n[{label}] FAILED: {exc}")
        return {"ok": False, "error": str(exc)}


def main():
    configure_logging()
    client = OpenAIClient()

    json_mode = run_mode("JSON mode", extract_with_json_mode, client)
    function_mode = run_mode("Function calling", extract_with_function_call, client)

    print("\n" + "=" * 40)
    print("Summary")
    print("=" * 40)
    print(f"JSON mode ok: {json_mode['ok']}")
    print(f"Function call ok: {function_mode['ok']}")

    if json_mode["ok"] and function_mode["ok"]:
        j = json_mode["result"]
        f = function_mode["result"]
        print(
            "\nBoth approaches returned valid events. "
            "Function calling gives you an explicit schema contract; "
            "JSON mode is simpler when you only need a shaped text response."
        )
        print(
            "Names match:",
            j.get("name") == f.get("name") and j.get("date") == f.get("date"),
        )


if __name__ == "__main__":
    main()
