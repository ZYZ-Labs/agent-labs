# Lab 14: Prompt-First Development

## Objectives

- Treat prompts as versioned assets, not inline strings.
- Load a prompt template from disk and render it with task variables.
- Use an LLM to generate a backend API specification and scaffold code from a short natural-language requirement.
- Persist generated artifacts so they can be inspected, diffed, and re-run.

## Structure

```
python/
  main.py              # driver: loads prompt, calls LLM, writes artifacts
  requirements.txt     # Python dependencies
  prompts/
    v1/
      spec_gen.txt     # prompt template v1
    v2/
      spec_gen.txt     # prompt template v2 (tighter instructions)
  generated/           # output directory (created at runtime)
```

## Run

```bash
cd python
# Copy the example env file and add your credentials
cp ../../../shared/config/.env.example .env 2>/dev/null || echo "Create a .env with OPENAI_API_KEY"
pip install -r requirements.txt
python main.py
```

## Environment Variables

| Variable          | Default                          | Description                  |
|-------------------|----------------------------------|------------------------------|
| `OPENAI_API_KEY`  | —                                | API key for non-local models |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1`      | API base URL                 |
| `OPENAI_MODEL`    | `gpt-4o-mini`                    | Model to use                 |
| `LOG_LEVEL`       | `INFO`                           | Logging verbosity            |

## Expected Output

The script prints the prompt version being used, sends the rendered prompt to the model, and writes two files under `generated/`:

- `api_spec.md` — OpenAPI-style spec derived from the requirement.
- `scaffold.py` — runnable FastAPI-style scaffold implementing the spec.

Console output shows a preview of both artifacts and confirms the output paths.
