import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


SYSTEM_PROMPT = """You are a helpful assistant that solves problems step by step.
You must follow this format exactly:

Thought: describe your reasoning
Action: tool_name(arg1, arg2, ...)
Observation: the result of the action (provided by the system)
...
Final Answer: the final answer

Available tools:
- calculator(expression: str) - evaluates a Python arithmetic expression safely
- finish(answer: str) - use when you have the final answer
"""


def calculator(expression: str) -> str:
    """A safe-ish calculator for demo purposes."""
    allowed = set("0123456789+-*/(). ")
    if not all(c in allowed for c in expression):
        return "Error: invalid characters"
    try:
        return str(eval(expression))  # noqa: S307
    except Exception as exc:
        return f"Error: {exc}"


def parse_action(text: str) -> tuple[str, list[str]] | None:
    match = re.search(r"Action:\s*(\w+)\((.*)\)", text)
    if not match:
        return None
    tool_name = match.group(1)
    args_str = match.group(2)
    # Simple split by comma, respecting quotes
    args = [a.strip().strip('"').strip("'") for a in args_str.split(",") if a.strip()]
    return tool_name, args


def run_react(client: OpenAIClient, question: str, max_steps: int = 10) -> str:
    messages = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": question},
    ]

    for step in range(max_steps):
        response = client.chat_completion(messages=messages, temperature=0.0, max_tokens=200)
        text = response["choices"][0]["message"]["content"]
        print(f"\n--- Step {step + 1} ---")
        print(text)

        if "Final Answer:" in text:
            return text.split("Final Answer:", 1)[1].strip()

        parsed = parse_action(text)
        if not parsed:
            observation = "Observation: I did not understand the action. Please use 'Action: tool_name(args)'."
        else:
            tool_name, args = parsed
            if tool_name == "calculator" and args:
                result = calculator(args[0])
                observation = f"Observation: {result}"
            elif tool_name == "finish" and args:
                return args[0]
            else:
                observation = f"Observation: unknown tool '{tool_name}'"

        print(observation)
        messages.append({"role": "assistant", "content": text})
        messages.append({"role": "user", "content": observation})

    return "Reached max steps without final answer."


def main():
    configure_logging()
    client = OpenAIClient()
    question = "What is (128 + 256) * 2 - 100?"
    print("Question:", question)
    answer = run_react(client, question)
    print("\nFinal Answer:", answer)


if __name__ == "__main__":
    main()
