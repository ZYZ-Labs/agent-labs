# Lab 02: LLM Parameters Explained

## Objectives

- Observe how each major LLM inference parameter changes model behavior.
- Understand when to use `temperature` vs `top_p`, `max_tokens`, `stop`, penalties, `seed`, and `response_format`.
- Build a reusable parameter explorer for your own prompts.

## Parameters Covered

- `temperature`
- `max_tokens`
- `top_p`
- `frequency_penalty`
- `presence_penalty`
- `stop`
- `seed`
- `response_format` (`json_object`)
- `n`

## Run

```bash
cp ../../../shared/config/.env.example .env
# Edit .env
```

Then run the language implementation of your choice.

## Expected Output

The script prints several parameterized completions side-by-side with token usage and finish reasons.
