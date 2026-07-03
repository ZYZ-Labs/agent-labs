# Lab 27: Observability

## Objectives

- Emit structured JSON logs from an agent run.
- Add OpenTelemetry-style spans around model calls and tool execution.
- Track prompt and completion token usage returned by the API.
- Correlate logs with a single trace ID for the whole request.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

Structured log lines showing:

- A root trace ID.
- Spans for `llm.call` and `tool.execute` with timing.
- Token usage fields (`prompt_tokens`, `completion_tokens`, `total_tokens`).
- The final agent response.
