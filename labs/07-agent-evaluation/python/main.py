import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


TEST_CASES = [
    {
        "input": "What port range is safe for user services?",
        "expected_keywords": ["1024", "65535"],
        "reference": "User services should use ports from 1024 to 65535.",
    },
    {
        "input": "Explain idempotency in one sentence.",
        "expected_keywords": ["same", "multiple", "result"],
        "reference": "Idempotency means calling an operation multiple times produces the same result.",
    },
]


def rule_check(answer: str, keywords: list[str]) -> dict:
    missing = [kw for kw in keywords if kw.lower() not in answer.lower()]
    return {"passed": not missing, "missing": missing}


def llm_judge(client: OpenAIClient, answer: str, reference: str) -> dict:
    prompt = f"""Rate how well the following answer matches the reference answer.
Answer: {answer}
Reference: {reference}
Respond with JSON only: {{"score": 1-10, "reason": "..."}}"""
    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        response_format={"type": "json_object"},
        max_tokens=200,
        temperature=0.0,
    )
    return json.loads(response["choices"][0]["message"]["content"])


def evaluate_agent(client: OpenAIClient, agent_fn) -> list[dict]:
    results = []
    for case in TEST_CASES:
        answer = agent_fn(client, case["input"])
        rule_result = rule_check(answer, case["expected_keywords"])
        judge_result = llm_judge(client, answer, case["reference"])
        results.append(
            {
                "input": case["input"],
                "answer": answer,
                "rule_passed": rule_result["passed"],
                "missing_keywords": rule_result["missing"],
                "judge_score": judge_result.get("score"),
                "judge_reason": judge_result.get("reason"),
            }
        )
    return results


def simple_agent(client: OpenAIClient, question: str) -> str:
    response = client.chat_completion(
        messages=[{"role": "user", "content": question}],
        max_tokens=200,
        temperature=0.0,
    )
    return response["choices"][0]["message"]["content"]


def main():
    configure_logging()
    client = OpenAIClient()
    results = evaluate_agent(client, simple_agent)

    passed = sum(1 for r in results if r["rule_passed"])
    total = len(results)
    avg_score = sum(r["judge_score"] or 0 for r in results) / total

    print(json.dumps(results, indent=2, ensure_ascii=False))
    print(f"\nRule checks passed: {passed}/{total}")
    print(f"Average judge score: {avg_score:.1f}/10")


if __name__ == "__main__":
    main()
