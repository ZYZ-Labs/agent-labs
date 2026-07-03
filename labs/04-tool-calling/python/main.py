import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "Get current weather for a city.",
            "parameters": {
                "type": "object",
                "properties": {
                    "city": {"type": "string", "description": "City name"},
                },
                "required": ["city"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "search_notes",
            "description": "Search project notes by keyword.",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search keyword"},
                },
                "required": ["query"],
            },
        },
    },
]


NOTES = [
    {"title": "MCP design", "content": "MCP uses Resources, Tools, Prompts, and Sampling primitives."},
    {"title": "LSP basics", "content": "LSP speaks JSON-RPC over stdio or sockets."},
    {"title": "Agent memory", "content": "Short-term memory lives in the context window; long-term in vectors."},
]


def get_weather(city: str) -> str:
    return json.dumps({"city": city, "temperature_c": 22, "condition": "sunny"})


def search_notes(query: str) -> str:
    results = [n for n in NOTES if query.lower() in n["title"].lower() or query.lower() in n["content"].lower()]
    return json.dumps(results, ensure_ascii=False)


TOOL_FUNCTIONS = {
    "get_weather": get_weather,
    "search_notes": search_notes,
}


def run_tool_agent(client: OpenAIClient, user_message: str, max_iterations: int = 5) -> str:
    messages = [{"role": "user", "content": user_message}]

    for _ in range(max_iterations):
        response = client.chat_completion(
            messages=messages,
            tools=TOOLS,
            tool_choice="auto",
            temperature=0.0,
            max_tokens=300,
        )
        message = response["choices"][0]["message"]
        messages.append(message)

        if not message.get("tool_calls"):
            return message["content"]

        for tool_call in message["tool_calls"]:
            name = tool_call["function"]["name"]
            args = json.loads(tool_call["function"]["arguments"])
            print(f"[Tool call] {name}({args})")
            func = TOOL_FUNCTIONS.get(name)
            result = func(**args) if func else json.dumps({"error": f"unknown tool {name}"})
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call["id"],
                    "name": name,
                    "content": result,
                }
            )
            print(f"[Tool result] {result}")

    return "Reached max iterations."


def main():
    configure_logging()
    client = OpenAIClient()
    question = "What's the weather in Shanghai? Also, find me notes about MCP."
    print("User:", question)
    answer = run_tool_agent(client, question)
    print("\nAssistant:", answer)


if __name__ == "__main__":
    main()
