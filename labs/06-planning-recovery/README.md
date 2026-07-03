# Lab 06: Planning and Self-Recovery

## Objectives

- Decompose a user request into a plan before acting.
- Detect execution failures and ask the model to retry with reflection.
- Implement a retry budget and fallback strategy.

## Scenario

The agent must generate a JSON configuration for a service. If validation fails, it reflects on the error and retries.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```
