# Lab 18: Multi-Agent Patterns

## Objectives

- Implement the Router + Worker + Aggregator pattern.
- Route a user request to one or more specialist agents.
- Aggregate specialist answers into a single coherent response.
- Gracefully fall back to keyword routing when no LLM key is available.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

Set `OPENAI_API_KEY` to use the LLM for routing, workers, and aggregation.

## Expected Output

The program prints the user request, the topics selected by the router, each specialist's partial answer, and the final aggregated answer.
