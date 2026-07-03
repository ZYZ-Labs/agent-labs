# Lab 12: Tree-sitter Parser for Prompt Context

## Objectives

- Parse Python source with [tree-sitter](https://tree-sitter.github.io/tree-sitter/).
- Extract function (and optionally class) names and their source ranges.
- Build a compact prompt context that an LLM can use to reason about a codebase.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

The script prints the AST node count, a list of discovered functions, and a
rendered prompt context block suitable for feeding into an LLM chat template.
