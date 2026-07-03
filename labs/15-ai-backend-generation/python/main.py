"""Lab 15: AI backend generation.

Asks an LLM to generate a CRUD backend module (models, routes, tests) from a
natural-language description, validates that each file parses/compiles, and
writes the generated files to disk.
"""

import ast
import logging
import os
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("agent-labs")

DESCRIPTION = (
    "A blog post resource with title (required string), body (required string), "
    "and published (boolean, default false). Provide CRUD endpoints for listing, "
    "creating, reading, updating, and deleting posts. Store posts in memory."
)

FILES = {
    "models.py": r"```python\s*models\.py\s*\n(.*?)\n```",
    "routes.py": r"```python\s*routes\.py\s*\n(.*?)\n```",
    "test_crud.py": r"```python\s*test_crud\.py\s*\n(.*?)\n```",
}


def load_prompt(description: str) -> str:
    prompt_path = Path(__file__).parent / "prompts" / "crud_gen.txt"
    template = prompt_path.read_text(encoding="utf-8")
    return template.replace("{{ description }}", description)


def extract_file(content: str, pattern: str) -> str:
    match = re.search(pattern, content, re.DOTALL)
    if not match:
        raise ValueError("Required code block not found in model response.")
    return match.group(1).strip()


def validate_python(source: str, filename: str) -> None:
    """Validate that source is valid Python by parsing and compiling it."""
    try:
        tree = ast.parse(source)
    except SyntaxError as exc:
        raise SyntaxError(f"{filename}: {exc}") from exc

    try:
        compile(tree, filename=filename, mode="exec")
    except SyntaxError as exc:
        raise SyntaxError(f"{filename}: {exc}") from exc

    # Basic sanity: generated routes should have a router, models should have classes.
    if filename == "routes.py":
        has_router = any(
            isinstance(node, (ast.Assign, ast.Call)) and
            getattr(node, "func", None) is not None and
            getattr(node.func, "id", "") == "APIRouter"
            for node in ast.walk(tree)
        )
        if not has_router:
            logger.warning("%s does not appear to define an APIRouter.", filename)


def generate_module(client: OpenAIClient, description: str) -> dict[str, str]:
    prompt = load_prompt(description)
    logger.info("Sending backend generation prompt (%d chars).", len(prompt))

    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        temperature=0.2,
        max_tokens=2000,
    )

    content = response["choices"][0]["message"]["content"]

    generated: dict[str, str] = {}
    for filename, pattern in FILES.items():
        generated[filename] = extract_file(content, pattern)
    return generated


def write_module(files: dict[str, str], output_dir: Path) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    for filename, source in files.items():
        (output_dir / filename).write_text(source + "\n", encoding="utf-8")
        logger.info("Wrote %s", output_dir / filename)


def main():
    configure_logging()
    client = OpenAIClient()

    try:
        files = generate_module(client, DESCRIPTION)
    except Exception as exc:
        logger.error("Generation failed: %s", exc)
        sys.exit(1)

    all_valid = True
    for filename, source in files.items():
        try:
            validate_python(source, filename)
            print(f"[OK] {filename} parses and compiles.")
        except SyntaxError as exc:
            all_valid = False
            print(f"[FAIL] {exc}")

    output_dir = Path(__file__).parent / "generated"
    try:
        write_module(files, output_dir)
    except OSError as exc:
        logger.error("Failed to write generated module: %s", exc)
        sys.exit(1)

    if not all_valid:
        logger.error("One or more generated files failed validation.")
        sys.exit(1)

    print(f"\nGenerated module written to: {output_dir.resolve()}")


if __name__ == "__main__":
    main()
