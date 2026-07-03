"""Lab 14: Prompt-first development.

Loads a versioned prompt template, renders it with a requirement, asks an LLM
to produce an API spec and a FastAPI scaffold, then writes the artifacts to disk.
"""

import json
import logging
import os
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("agent-labs")

REQUIREMENT = (
    "Build an API for a task manager. Users can create a task with a title and "
    "optional description, list all tasks, and mark a task as complete. "
    "Store tasks in memory only."
)

PROMPT_VERSION = os.getenv("PROMPT_VERSION", "v2")


def load_prompt_template(version: str) -> str:
    prompt_path = Path(__file__).parent / "prompts" / version / "spec_gen.txt"
    if not prompt_path.exists():
        available = [p.name for p in (Path(__file__).parent / "prompts").iterdir() if p.is_dir()]
        raise FileNotFoundError(
            f"Prompt template not found for version '{version}'. "
            f"Available versions: {available or 'none'}"
        )
    return prompt_path.read_text(encoding="utf-8")


def render_prompt(template: str, requirement: str) -> str:
    return template.replace("{{ requirement }}", requirement)


def extract_code_blocks(content: str) -> tuple[str, str]:
    """Extract the first markdown and first python code blocks."""
    md_match = re.search(r"```markdown\s*\n(.*?)\n```", content, re.DOTALL)
    py_match = re.search(r"```python\s*\n(.*?)\n```", content, re.DOTALL)

    if not md_match or not py_match:
        raise ValueError("Model response did not contain both required markdown and python code blocks.")

    return md_match.group(1).strip(), py_match.group(1).strip()


def generate_artifacts(client: OpenAIClient, requirement: str, version: str) -> tuple[str, str]:
    template = load_prompt_template(version)
    prompt = render_prompt(template, requirement)

    logger.info("Using prompt version: %s", version)
    logger.info("Sending %d prompt characters to model.", len(prompt))

    response = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        temperature=0.2,
        max_tokens=1500,
    )

    content = response["choices"][0]["message"]["content"]
    spec, scaffold = extract_code_blocks(content)
    return spec, scaffold


def write_artifacts(spec: str, scaffold: str, output_dir: Path) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)

    spec_path = output_dir / "api_spec.md"
    scaffold_path = output_dir / "scaffold.py"

    spec_path.write_text(spec + "\n", encoding="utf-8")
    scaffold_path.write_text(scaffold + "\n", encoding="utf-8")

    logger.info("Wrote artifacts to %s", output_dir)
    logger.info("  - %s", spec_path.name)
    logger.info("  - %s", scaffold_path.name)


def preview(text: str, max_chars: int = 400) -> str:
    if len(text) <= max_chars:
        return text
    return text[:max_chars].rstrip() + "\n..."


def main():
    configure_logging()
    client = OpenAIClient()

    try:
        spec, scaffold = generate_artifacts(client, REQUIREMENT, PROMPT_VERSION)
    except Exception as exc:
        logger.error("Failed to generate artifacts: %s", exc)
        sys.exit(1)

    output_dir = Path(__file__).parent / "generated"
    try:
        write_artifacts(spec, scaffold, output_dir)
    except OSError as exc:
        logger.error("Failed to write artifacts: %s", exc)
        sys.exit(1)

    print("\n=== Generated API Spec Preview ===")
    print(preview(spec))
    print("\n=== Generated Scaffold Preview ===")
    print(preview(scaffold))
    print(f"\nArtifacts written to: {output_dir.resolve()}")


if __name__ == "__main__":
    main()
