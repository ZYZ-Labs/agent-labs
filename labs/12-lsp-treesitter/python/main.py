"""Use tree-sitter to parse Python, extract function names, and build prompt context."""

import sys
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import configure_logging


SAMPLE_CODE = '''\
"""A tiny module for demonstration."""

import json
from datetime import datetime


def greet(name: str) -> str:
    """Return a friendly greeting."""
    return f"Hello, {name}!"


class Calculator:
    """Simple calculator."""

    def add(self, a: float, b: float) -> float:
        return a + b

    def subtract(self, a: float, b: float) -> float:
        return a - b


def main():
    calc = Calculator()
    print(greet("world"))
    print(calc.add(1, 2))
'''


def load_parser() -> Any:
    """Import tree-sitter and create a Python parser."""
    try:
        from tree_sitter import Language, Parser
        from tree_sitter_python import language
    except ImportError as exc:
        raise ImportError(
            "tree-sitter and tree-sitter-python are required. "
            "Install: pip install -r requirements.txt"
        ) from exc

    parser = Parser(Language(language()))
    return parser


def walk_tree(node, depth: int = 0) -> int:
    """Return total node count by walking the AST."""
    count = 1
    for child in node.children:
        count += walk_tree(child, depth + 1)
    return count


def extract_definitions(tree, source: bytes) -> list[dict[str, Any]]:
    """Extract function and class definitions with source ranges."""
    definitions: list[dict[str, Any]] = []

    def visit(node):
        if node.type in ("function_definition", "class_definition"):
            name_node = next(
                (c for c in node.children if c.type == "identifier"),
                None,
            )
            if name_node:
                start_line = source[: name_node.start_byte].decode("utf-8").count("\n")
                definitions.append(
                    {
                        "kind": node.type.replace("_definition", ""),
                        "name": source[name_node.start_byte : name_node.end_byte].decode("utf-8"),
                        "line": start_line + 1,
                        "start_byte": node.start_byte,
                        "end_byte": node.end_byte,
                    }
                )
        for child in node.children:
            visit(child)

    visit(tree.root_node)
    return definitions


def build_prompt_context(source: str, definitions: list[dict[str, Any]]) -> str:
    """Build a concise prompt context describing the discovered symbols."""
    lines = [
        "You are reviewing the following Python module.",
        "",
        "## Symbols",
    ]
    for d in definitions:
        kind = d["kind"].capitalize()
        lines.append(f"- {kind} `{d['name']}` at line {d['line']} (bytes {d['start_byte']}-{d['end_byte']})")

    lines.extend([
        "",
        "## Source",
        "```python",
        source,
        "```",
        "",
        "Please summarize what this module does.",
    ])
    return "\n".join(lines)


def main():
    configure_logging()
    try:
        parser = load_parser()
    except ImportError as exc:
        print(f"Error: {exc}")
        sys.exit(1)

    source_bytes = SAMPLE_CODE.encode("utf-8")
    tree = parser.parse(source_bytes)

    print(f"AST node count: {walk_tree(tree.root_node)}")

    definitions = extract_definitions(tree, source_bytes)
    print("\nDiscovered definitions:")
    for d in definitions:
        print(f"  [{d['kind']}] {d['name']} @ line {d['line']}")

    context = build_prompt_context(SAMPLE_CODE, definitions)
    print("\n" + "=" * 50)
    print("Generated prompt context")
    print("=" * 50)
    print(context)


if __name__ == "__main__":
    main()
