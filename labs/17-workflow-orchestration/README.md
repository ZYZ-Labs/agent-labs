# Lab 17: Workflow Orchestration

## Objectives

- Build a minimal DAG workflow engine from scratch.
- Define tasks with dependencies, retries, and collected results.
- Execute tasks in topological order.
- Observe retry behavior on a flaky task.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

Set `OPENAI_API_KEY` to use the LLM for sentiment analysis and summarization. Without it, the lab falls back to deterministic outputs.

## Expected Output

The engine prints the execution order, retries the flaky quality-check task, and finally prints a JSON blob with results for `fetch`, `sentiment`, `quality`, and `summary`.
