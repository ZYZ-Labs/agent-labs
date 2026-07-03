# Lab 19: Long Task Reliability

## Objectives

- Simulate a long-running task with checkpoints and resume capability.
- Use an idempotency key to avoid duplicate work.
- Retry failing steps with exponential backoff.
- Enforce a per-step timeout.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

Set `OPENAI_API_KEY` to use the LLM for the mock data-fetch step. Without it, a deterministic fallback is used.

## Expected Output

The task prints its idempotency key, retries the flaky `process` step twice, completes, and then demonstrates idempotency by re-running and skipping completed steps.
