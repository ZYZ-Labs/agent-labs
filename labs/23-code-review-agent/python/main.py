"""Code review agent pipeline.

Reads a Python file, runs static/style checks, sends to an LLM for review,
and aggregates a report.
"""
import argparse
import json
import logging
import py_compile
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("code-review-agent")


def _find_ruff() -> str | None:
    if ruff := shutil.which("ruff"):
        return ruff
    candidate = Path(sys.executable).with_name("ruff")
    if candidate.exists():
        return str(candidate)
    candidate_exe = Path(sys.executable).with_name("ruff.exe")
    if candidate_exe.exists():
        return str(candidate_exe)
    return None


def run_syntax_check(path: Path) -> dict:
    try:
        py_compile.compile(str(path), doraise=True)
        return {"ok": True, "issues": []}
    except py_compile.PyCompileError as exc:
        return {"ok": False, "issues": [f"Syntax error: {exc}"]}


def run_ruff(path: Path) -> dict:
    ruff_bin = _find_ruff()
    if not ruff_bin:
        logger.warning("ruff not installed; skipping style check")
        return {"ok": True, "issues": ["ruff not installed"]}
    try:
        result = subprocess.run(
            [ruff_bin, "check", str(path), "--output-format", "json"],
            capture_output=True,
            text=True,
            check=False,
        )
        issues = []
        if result.stdout.strip():
            try:
                parsed = json.loads(result.stdout)
                for item in parsed:
                    location = item.get("location", {})
                    issues.append(
                        f"Line {location.get('row')}: {item.get('code')} - {item.get('message')}"
                    )
            except json.JSONDecodeError:
                issues.append(result.stdout.strip())
        return {"ok": len(issues) == 0, "issues": issues}
    except FileNotFoundError:
        logger.warning("ruff not installed; skipping style check")
        return {"ok": True, "issues": ["ruff not installed"]}


def llm_review(path: Path, source: str, checks: dict, client: OpenAIClient | None) -> str:
    if not client:
        return "LLM review skipped (no API key). Manual review recommended."

    prompt = f"""Review the following Python file for style, bugs, and maintainability.

File: {path.name}

Static checks:
- Syntax OK: {checks['syntax']['ok']}
- Style issues: {checks['ruff']['issues']}

Source code:
```python
{source}
```

Provide a concise review with:
1. Critical issues
2. Suggestions
3. Overall verdict (OK / Needs work)."""

    resp = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        temperature=0.2,
        max_tokens=400,
    )
    return resp["choices"][0]["message"]["content"].strip()


def review_file(path: Path, client: OpenAIClient | None) -> dict:
    logger.info("Reviewing %s", path)
    source = path.read_text(encoding="utf-8")
    checks = {
        "syntax": run_syntax_check(path),
        "ruff": run_ruff(path),
    }
    review = llm_review(path, source, checks, client)
    ruff_issues = [i for i in checks["ruff"]["issues"] if i != "ruff not installed"]
    return {
        "file": str(path),
        "checks": checks,
        "llm_review": review,
        "verdict": "needs_work" if not checks["syntax"]["ok"] or ruff_issues else "ok",
    }


def main() -> None:
    configure_logging()
    parser = argparse.ArgumentParser(description="Code review agent")
    parser.add_argument(
        "file", nargs="?", default="sample_code.py", help="Python file to review"
    )
    args = parser.parse_args()

    client = None
    try:
        client = OpenAIClient()
    except ValueError as exc:
        logger.warning("LLM client disabled: %s", exc)

    target = Path(args.file)
    if not target.exists():
        logger.error("File not found: %s", target)
        sys.exit(1)

    report = review_file(target, client)
    print(json.dumps(report, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
