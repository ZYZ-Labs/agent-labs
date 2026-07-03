# Lab 20: Human-in-the-Loop

## Objectives

- Build a workflow that pauses for human approval before a destructive action.
- Persist workflow state so it can resume after restart.
- Execute only after explicit approval; reject cleanly otherwise.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

The program asks for approval on the terminal. Answer `y` to approve or `n` to reject.

To reset the saved state and run again:

```bash
python main.py --reset
```

## Expected Output

The workflow describes the destructive action, persists state, waits for input, and either executes the action or reports rejection. On a second run after approval but before execution, it resumes and executes.
