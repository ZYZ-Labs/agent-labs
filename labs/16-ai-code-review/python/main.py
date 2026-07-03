"""Lab 16: AI-assisted code review.

Reads a source file, sends it to an LLM with a structured review prompt, parses
the JSON response, and prints a categorized report.
"""

import json
import logging
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("agent-labs")

REVIEW_PROMPT = """You are a meticulous code reviewer. Review the following Python source file and return ONLY a JSON object.

Rules for the JSON object:
- Top-level keys must be exactly: "security", "style", "logic".
- Each value is a list of findings. An empty list is allowed.
- Each finding is an object with these keys:
  - "severity": one of "HIGH", "MEDIUM", "LOW".
  - "line": integer line number, or null if not applicable.
  - "message": concise description of the issue.
  - "suggestion": concrete recommendation to fix it.

Be strict but fair. Focus on real issues, not nitpicks.

```python
{code}
```
"""

CATEGORIES = ["security", "style", "logic"]
SEVERITY_ORDER = {"HIGH": 0, "MEDIUM": 1, "LOW": 2}


def read_source_file(path: Path) -> str:
    if not path.exists():
        raise FileNotFoundError(f"Source file not found: {path}")
    if not path.is_file():
        raise ValueError(f"Path is not a file: {path}")
    return path.read_text(encoding="utf-8")


def review_code(client: OpenAIClient, source: str) -> dict:
    prompt = REVIEW_PROMPT.format(code=source)
    logger.info("Sending code review prompt (%d chars of code).", len(source))

    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        temperature=0.2,
        max_tokens=1200,
        response_format={"type": "json_object"},
    )

    content = response["choices"][0]["message"]["content"]
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise ValueError(f"Model returned invalid JSON: {exc}\nRaw content:\n{content}") from exc


def validate_report(report: dict) -> None:
    if not isinstance(report, dict):
        raise ValueError("Review report is not a JSON object.")

    missing = [cat for cat in CATEGORIES if cat not in report]
    if missing:
        raise ValueError(f"Review report missing required categories: {missing}")

    for category in CATEGORIES:
        if not isinstance(report[category], list):
            raise ValueError(f"Category '{category}' must be a list.")
        for idx, finding in enumerate(report[category]):
            if not isinstance(finding, dict):
                raise ValueError(f"Finding {idx} in '{category}' is not an object.")
            for key in ("severity", "message", "suggestion"):
                if key not in finding:
                    raise ValueError(f"Finding {idx} in '{category}' missing key '{key}'.")
            if finding["severity"] not in SEVERITY_ORDER:
                raise ValueError(
                    f"Finding {idx} in '{category}' has invalid severity '{finding['severity']}'."
                )


def print_report(file_path: Path, line_count: int, report: dict) -> None:
    categories_present = [cat for cat in CATEGORIES if report.get(cat)]

    print(f"\nReview: {file_path}")
    print(f"Lines: {line_count}")
    print(f"Categories: {', '.join(categories_present) or 'none with findings'}")

    total = 0
    counts = {"HIGH": 0, "MEDIUM": 0, "LOW": 0}

    for category in CATEGORIES:
        findings = report.get(category, [])
        if not findings:
            continue
        print(f"\n{category.upper()}")
        for finding in sorted(findings, key=lambda f: SEVERITY_ORDER.get(f["severity"], 99)):
            total += 1
            counts[finding["severity"]] += 1
            line = finding.get("line")
            line_info = f" at line {line}" if line is not None else ""
            print(f"  - {finding['severity']}{line_info}: {finding['message']}")
            print(f"    Suggestion: {finding['suggestion']}")

    print(f"\nSummary: {total} issue(s) found. {counts['HIGH']} high, {counts['MEDIUM']} medium, {counts['LOW']} low.")


def main():
    configure_logging()
    client = OpenAIClient()

    target = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).parent / "sample_code.py"

    try:
        source = read_source_file(target)
    except (FileNotFoundError, ValueError) as exc:
        logger.error("%s", exc)
        sys.exit(1)

    try:
        report = review_code(client, source)
        validate_report(report)
    except Exception as exc:
        logger.error("Review failed: %s", exc)
        sys.exit(1)

    print_report(target, len(source.splitlines()), report)


if __name__ == "__main__":
    main()
