import os
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

OUTPUT_DIR = Path(__file__).resolve().parents[1] / "output"

DEFAULT_REQUIREMENTS = """Build a simple REST API for a task management service.
Users should be able to create, list, update, and delete tasks.
Each task has a title, description, status (todo/in_progress/done), and due date.
Store data in memory; no database is required.
Add basic input validation."""


def sanitize_filename(text: str) -> str:
    return re.sub(r"[^a-zA-Z0-9_\-]", "_", text)[:50]


def generate_design_doc(client: OpenAIClient, requirements: str) -> str:
    prompt = f"""You are a senior backend engineer. Write a concise design document (Markdown) for the following requirements.

Requirements:
{requirements}

Include:
- Overview
- Endpoints (method, path, description)
- Data model
- Assumptions and constraints

Respond with Markdown only."""
    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        max_tokens=1500,
        temperature=0.2,
    )
    return response["choices"][0]["message"].get("content", "")


def generate_api_code(client: OpenAIClient, requirements: str, design_doc: str) -> str:
    prompt = f"""You are a senior backend engineer. Implement a runnable FastAPI application in Python for the requirements below.

Requirements:
{requirements}

Design document:
{design_doc}

Guidelines:
- Use FastAPI and Pydantic.
- Store data in memory.
- Include input validation.
- Do not include instructions or explanations outside the code.
- Output a single Python file named api_code.py.

Respond with the full Python source code only (no Markdown fences)."""
    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        max_tokens=2000,
        temperature=0.2,
    )
    return _strip_markdown_fences(response["choices"][0]["message"].get("content", ""))


def generate_tests(client: OpenAIClient, api_code: str) -> str:
    prompt = f"""You are a QA engineer. Write pytest tests for the following FastAPI application.

Application code:
{api_code}

Guidelines:
- Use fastapi.testclient.TestClient.
- Cover create, list, update, and delete endpoints.
- Include at least one validation failure test.
- Output a single Python file named test_api.py.

Respond with the full Python test source code only (no Markdown fences)."""
    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        max_tokens=2000,
        temperature=0.2,
    )
    return _strip_markdown_fences(response["choices"][0]["message"].get("content", ""))


def generate_review_report(client: OpenAIClient, requirements: str, design_doc: str, api_code: str, tests: str) -> str:
    prompt = f"""You are a staff engineer reviewing the following backend artifact bundle.

Requirements:
{requirements}

Design Document:
{design_doc}

API Code:
{api_code}

Tests:
{tests}

Produce a review report (Markdown) with:
- Summary
- What was done well
- Risks and concerns
- Actionable recommendations
- Pass/needs-work verdict

Respond with Markdown only."""
    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        max_tokens=1500,
        temperature=0.2,
    )
    return response["choices"][0]["message"].get("content", "")


def _strip_markdown_fences(text: str) -> str:
    text = text.strip()
    if text.startswith("```"):
        lines = text.splitlines()
        if lines[0].startswith("```"):
            lines = lines[1:]
        if lines and lines[-1].startswith("```"):
            lines = lines[:-1]
        text = "\n".join(lines)
    return text.strip()


def write_artifact(name: str, content: str) -> Path:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    path = OUTPUT_DIR / name
    path.write_text(content, encoding="utf-8")
    return path


def main():
    configure_logging()
    client = OpenAIClient()

    requirements = os.getenv("REQUIREMENTS", DEFAULT_REQUIREMENTS)
    print("Requirements:")
    print(requirements)
    print()

    print("Generating design doc...")
    design_doc = generate_design_doc(client, requirements)
    design_path = write_artifact("design_doc.md", design_doc)
    print(f"  Wrote {design_path}")

    print("Generating API code...")
    api_code = generate_api_code(client, requirements, design_doc)
    api_path = write_artifact("api_code.py", api_code)
    print(f"  Wrote {api_path}")

    print("Generating tests...")
    tests = generate_tests(client, api_code)
    tests_path = write_artifact("test_api.py", tests)
    print(f"  Wrote {tests_path}")

    print("Generating review report...")
    review = generate_review_report(client, requirements, design_doc, api_code, tests)
    review_path = write_artifact("review_report.md", review)
    print(f"  Wrote {review_path}")

    print("\nCapstone artifacts generated successfully.")


if __name__ == "__main__":
    main()
