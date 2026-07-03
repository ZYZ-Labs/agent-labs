# Lab 24: SRE Agent

## Objectives

- Read an application log file.
- Use an LLM to diagnose issues and propose remediation commands.
- Ask a human operator for confirmation before executing commands.
- Only execute commands that match an allowlist.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

To analyze a different log:

```bash
python main.py /var/log/myapp.log
```

Set `OPENAI_API_KEY` to use the LLM for diagnosis. Without it, a deterministic fallback diagnosis is used.

## Expected Output

The agent prints the number of log entries, a diagnosis, proposed remediation commands, prompts for approval, and then executes or skips the commands.
