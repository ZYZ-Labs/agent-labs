# Lab 25: Agent Security

## Objectives

- Detect common prompt-injection patterns before sending them to the model.
- Sanitize model outputs to reduce risky markup and script tags.
- Redact PII such as emails, phone numbers, and credit-card numbers.
- Enforce a tool allow-list so the agent cannot execute unapproved tools.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

The program inspects incoming user messages and model outputs:

- A flagged injection attempt is blocked with a warning.
- A benign request passes through and receives a model response.
- PII in the user prompt is redacted.
- The model output is sanitized.
- A tool call outside the allow-list is rejected.
