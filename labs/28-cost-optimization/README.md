# Lab 28: Cost Optimization

## Objectives

- Compare per-model cost estimates for a fixed input/output token budget.
- Implement a simple prompt-caching strategy by reusing a system prefix.
- Route requests to a cheaper or more capable model based on task complexity.
- Estimate token spend before making the actual API call.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

A cost comparison table, a routing decision for each sample task, an estimated spend, and a cached-prompt example showing reduced token count.
