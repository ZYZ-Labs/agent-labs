import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

# Approximate cost per 1M tokens (input, output). Update with current pricing.
MODEL_PRICING = {
    "gpt-4o": {"input": 5.00, "output": 15.00},
    "gpt-4o-mini": {"input": 0.15, "output": 0.60},
    "gpt-3.5-turbo": {"input": 0.50, "output": 1.50},
}


def estimate_cost(model: str, prompt_tokens: int, completion_tokens: int) -> float:
    pricing = MODEL_PRICING.get(model, {"input": 0.0, "output": 0.0})
    return (prompt_tokens * pricing["input"] + completion_tokens * pricing["output"]) / 1_000_000


def compare_costs(prompt_tokens: int, completion_tokens: int) -> list[dict]:
    return [
        {
            "model": model,
            "estimated_cost_usd": round(estimate_cost(model, prompt_tokens, completion_tokens), 6),
        }
        for model in MODEL_PRICING
    ]


def choose_model(task_description: str) -> str:
    # Simple heuristic router.
    desc = task_description.lower()
    complex_signals = ["architecture", "design doc", "refactor", "complex", "multistep", "review"]
    if any(signal in desc for signal in complex_signals):
        return "gpt-4o"
    return "gpt-4o-mini"


def count_tokens(text: str, model: str = "gpt-4o-mini") -> int:
    try:
        import tiktoken

        encoder = tiktoken.encoding_for_model(model)
        return len(encoder.encode(text))
    except Exception:
        # Fallback: rough character count.
        return len(text.split())


def run_cached_prompt_agent(client: OpenAIClient, user_message: str) -> dict:
    system_prefix = (
        "You are a concise coding assistant. Always answer in one sentence. "
        "Prefer short variable names and simple algorithms."
    )

    # Estimate tokens with and without cache reuse.
    full_prompt = system_prefix + "\n" + user_message
    full_tokens = count_tokens(full_prompt)
    user_tokens = count_tokens(user_message)

    response = client.chat_completion(
        messages=[
            {"role": "system", "content": system_prefix},
            {"role": "user", "content": user_message},
        ],
        max_tokens=200,
        temperature=0.0,
    )
    content = response["choices"][0]["message"].get("content", "")
    completion_tokens = (response.get("usage") or {}).get("completion_tokens", count_tokens(content))

    # Treat the system prefix as cached: only user tokens are billed as new input.
    estimated_cost_cached = estimate_cost(client.model, user_tokens, completion_tokens)
    estimated_cost_uncached = estimate_cost(client.model, full_tokens, completion_tokens)

    return {
        "model": client.model,
        "user_message": user_message,
        "full_prompt_tokens": full_tokens,
        "new_input_tokens": user_tokens,
        "completion_tokens": completion_tokens,
        "estimated_cost_cached_usd": round(estimated_cost_cached, 6),
        "estimated_cost_uncached_usd": round(estimated_cost_uncached, 6),
        "response": content,
    }


def main():
    configure_logging()
    client = OpenAIClient()

    print("Cost comparison for 1000 input + 500 output tokens:")
    for row in compare_costs(1000, 500):
        print(f"  {row['model']}: ${row['estimated_cost_usd']:.6f}")

    tasks = [
        "Summarize this paragraph in one sentence.",
        "Generate an architecture design doc for a payment gateway.",
        "Refactor this Python function to use async IO.",
    ]
    print("\nRouting decisions:")
    for task in tasks:
        model = choose_model(task)
        print(f"  [{model}] {task}")

    user_message = "What is the capital of France?"
    print(f"\nCached prompt example:\nUser: {user_message}")
    result = run_cached_prompt_agent(client, user_message)
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
