import json
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

TEST_CASES_PATH = Path(__file__).with_name("test_cases.json")


def load_test_cases(path: Path) -> list[dict]:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def rule_check(answer: str, case: dict, latency_ms: float) -> dict:
    checks = {}

    keywords = case.get("expected_keywords", [])
    missing = [kw for kw in keywords if kw.lower() not in answer.lower()]
    checks["keywords"] = {"passed": not missing, "missing": missing}

    if case.get("expect_json"):
        try:
            json.loads(answer)
            checks["json"] = {"passed": True}
        except json.JSONDecodeError:
            checks["json"] = {"passed": False, "error": "not valid JSON"}

    max_latency = case.get("max_latency_ms")
    if max_latency is not None:
        checks["latency"] = {"passed": latency_ms <= max_latency, "latency_ms": latency_ms}

    all_passed = all(c["passed"] for c in checks.values())
    return {"passed": all_passed, "checks": checks}


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
    try:
        return json.loads(response["choices"][0]["message"]["content"])
    except (json.JSONDecodeError, KeyError) as exc:
        return {"score": 0, "reason": f"Failed to parse judge response: {exc}"}


def run_agent(client: OpenAIClient, question: str) -> str:
    response = client.chat_completion(
        messages=[{"role": "user", "content": question}],
        max_tokens=200,
        temperature=0.0,
    )
    return response["choices"][0]["message"].get("content", "")


def evaluate(client: OpenAIClient, cases: list[dict]) -> dict:
    results = []
    for case in cases:
        start = time.perf_counter()
        answer = run_agent(client, case["input"])
        latency_ms = (time.perf_counter() - start) * 1000

        rule_result = rule_check(answer, case, latency_ms)
        judge_result = llm_judge(client, answer, case["reference"])

        results.append(
            {
                "id": case["id"],
                "input": case["input"],
                "answer": answer,
                "rule_passed": rule_result["passed"],
                "rule_details": rule_result["checks"],
                "judge_score": judge_result.get("score"),
                "judge_reason": judge_result.get("reason"),
            }
        )

    total = len(results)
    rule_passed = sum(1 for r in results if r["rule_passed"])
    judge_scores = [r["judge_score"] for r in results if isinstance(r["judge_score"], (int, float))]
    avg_score = sum(judge_scores) / len(judge_scores) if judge_scores else 0

    return {
        "total": total,
        "rule_pass_rate": rule_passed / total if total else 0,
        "average_judge_score": avg_score,
        "cases": results,
    }


def main():
    configure_logging()
    client = OpenAIClient()
    cases = load_test_cases(TEST_CASES_PATH)
    report = evaluate(client, cases)
    print(json.dumps(report, indent=2, ensure_ascii=False))
    print(f"\nRule pass rate: {report['rule_pass_rate']:.0%}")
    print(f"Average judge score: {report['average_judge_score']:.1f}/10")


if __name__ == "__main__":
    main()
