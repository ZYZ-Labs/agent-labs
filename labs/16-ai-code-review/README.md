# Lab 16: AI-Assisted Code Review

## Objectives

- Build a script that reads a source file and sends it to an LLM with a structured review prompt.
- Instruct the model to return a JSON object with categorized findings (security, style, logic).
- Parse the JSON response and print a human-readable report.
- Surface missing credentials or malformed output gracefully.

## Structure

```
python/
  main.py              # driver: reads file, reviews with LLM, prints report
  requirements.txt     # Python dependencies
  sample_code.py       # example source file to review
```

## Run

```bash
cd python
# Copy the example env file and add your credentials
cp ../../../shared/config/.env.example .env 2>/dev/null || echo "Create a .env with OPENAI_API_KEY"
pip install -r requirements.txt
python main.py
```

To review a different file, pass its path as an argument:

```bash
python main.py /path/to/file.py
```

## Environment Variables

| Variable          | Default                          | Description                  |
|-------------------|----------------------------------|------------------------------|
| `OPENAI_API_KEY`  | —                                | API key for non-local models |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1`      | API base URL                 |
| `OPENAI_MODEL`    | `gpt-4o-mini`                    | Model to use                 |
| `LOG_LEVEL`       | `INFO`                           | Logging verbosity            |

## Expected Output

The script prints a review report grouped by category, for example:

```
Review: sample_code.py
Lines: 24
Categories: security, style, logic

SECURITY
  - HIGH at line 12: Hardcoded password in source.
    Suggestion: Load secrets from environment variables.

STYLE
  - LOW at line 5: Function name uses camelCase.
    Suggestion: Use snake_case per PEP 8.

LOGIC
  - MEDIUM at line 19: Division may raise ZeroDivisionError.
    Suggestion: Guard against zero before dividing.

Summary: 3 issue(s) found. 1 high, 1 medium, 1 low.
```

If the model returns malformed JSON, the script reports the parsing error and exits with a non-zero code.
