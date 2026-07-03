# Lab 03: Handwritten ReAct Loop

## Objectives

- Implement the ReAct (Reasoning + Acting) loop without any agent framework.
- Parse model output into `Thought` / `Action` / `Observation` steps.
- Use a simple calculator tool and stop when the task is done.

## Scenario

The agent answers arithmetic questions by reasoning step-by-step and using a `calculator` tool when needed.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

A trace of thoughts, actions, observations, and the final answer.
