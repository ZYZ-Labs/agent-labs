# Lab 15: AI Backend Generation

## Objectives

- Prompt an LLM to generate a complete CRUD-like backend module from a natural-language description.
- Receive model + route + test files from the model in a structured format.
- Validate that generated Python code parses (and optionally compiles) without executing it.
- Persist the generated module so it can be inspected and iterated on.

## Structure

```
python/
  main.py              # driver: describes resource, calls LLM, validates, writes files
  requirements.txt     # Python dependencies
  prompts/
    crud_gen.txt       # prompt template
  generated/           # output directory (created at runtime)
    models.py
    routes.py
    test_crud.py
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

The script prints:

- The parsed description.
- Whether each generated file passed the parser/compile check.
- The paths of all written files.

If validation fails, the script exits with a non-zero code and reports the offending file plus the syntax error.
