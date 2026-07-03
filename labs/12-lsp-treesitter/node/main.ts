// @ts-ignore
import Parser from "tree-sitter";
// @ts-ignore
import Python from "tree-sitter-python";

const SAMPLE_CODE = `\
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
`;

function loadParser(): any {
  try {
    const parser = new Parser();
    parser.setLanguage(Python);
    return parser;
  } catch (exc: any) {
    throw new Error(
      `tree-sitter and tree-sitter-python are required. Install: npm install`
    );
  }
}

function walkTree(node: any, depth = 0): number {
  let count = 1;
  for (const child of node.children) {
    count += walkTree(child, depth + 1);
  }
  return count;
}

function extractDefinitions(tree: any, source: Buffer): any[] {
  const definitions: any[] = [];

  function visit(node: any): void {
    if (node.type === "function_definition" || node.type === "class_definition") {
      const nameNode = node.children.find((c: any) => c.type === "identifier");
      if (nameNode) {
        const startLine = source.slice(0, nameNode.startByte).toString("utf8").split("\n").length;
        definitions.push({
          kind: node.type.replace("_definition", ""),
          name: source.slice(nameNode.startByte, nameNode.endByte).toString("utf8"),
          line: startLine,
          start_byte: node.startByte,
          end_byte: node.endByte,
        });
      }
    }
    for (const child of node.children) {
      visit(child);
    }
  }

  visit(tree.rootNode);
  return definitions;
}

function buildPromptContext(source: string, definitions: any[]): string {
  const lines = [
    "You are reviewing the following Python module.",
    "",
    "## Symbols",
  ];
  for (const d of definitions) {
    const kind = d.kind.charAt(0).toUpperCase() + d.kind.slice(1);
    lines.push(
      `- ${kind} \`${d.name}\` at line ${d.line} (bytes ${d.start_byte}-${d.end_byte})`
    );
  }
  lines.push("", "## Source", "```python", source, "```", "", "Please summarize what this module does.");
  return lines.join("\n");
}

async function main() {
  let parser: any;
  try {
    parser = loadParser();
  } catch (exc: any) {
    console.error(`Error: ${exc.message}`);
    process.exit(1);
  }

  const sourceBytes = Buffer.from(SAMPLE_CODE, "utf8");
  const tree = parser.parse(sourceBytes);

  console.log(`AST node count: ${walkTree(tree.rootNode)}`);

  const definitions = extractDefinitions(tree, sourceBytes);
  console.log("\nDiscovered definitions:");
  for (const d of definitions) {
    console.log(`  [${d.kind}] ${d.name} @ line ${d.line}`);
  }

  const context = buildPromptContext(SAMPLE_CODE, definitions);
  console.log("\n" + "=".repeat(50));
  console.log("Generated prompt context");
  console.log("=".repeat(50));
  console.log(context);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
