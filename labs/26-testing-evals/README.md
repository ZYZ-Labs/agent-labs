# Lab 26: Testing & Evaluations

## Objectives

- Load a small set of test cases from a JSON file.
- Run a simple agent against every test case.
- Apply deterministic rule checks (keyword, format, latency).
- Use an LLM-as-judge to score answers against a reference.
- Report pass rate, average score, and a per-case breakdown.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

A JSON report with rule-check results, judge scores, pass rate, and average score for the test set.
