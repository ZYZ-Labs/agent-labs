# Lab 23: Code Review Agent

## Objectives

- Read a Python file.
- Run syntax and style/static checks.
- Send the code plus check results to an LLM for review.
- Aggregate everything into a single JSON report.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

To review a different file:

```bash
python main.py path/to/file.py
```

Set `OPENAI_API_KEY` to include an LLM review. Without it, the report still contains syntax and style findings.

## Expected Output

A JSON report with `file`, `checks` (syntax and ruff), `llm_review`, and a top-level `verdict`.
