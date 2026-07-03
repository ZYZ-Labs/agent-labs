# Lab 08: Function Calling vs JSON Mode

## Objectives

- Compare OpenAI **Function Calling** (`tools`) with **JSON mode** (`response_format={"type": "json_object"}`).
- Parse structured output from both approaches.
- Validate the result against a schema and handle malformed / partial JSON gracefully.
- Decide when to prefer explicit tool calls versus a constrained JSON response.

## Scenario

Extract an event from free-form user text:

```json
{
  "name": "string",
  "date": "string",
  "location": "string",
  "participants": ["string"]
}
```

The lab runs the same prompt twice — once with JSON mode and once with a single
`extract_event` tool — then compares the parsed arguments.

## Run

```bash
cd python
pip install -r requirements.txt
# create a .env file with OPENAI_API_KEY if needed
python main.py
```

## Expected Output

The program prints the raw model output for both modes, parses the JSON, validates
required fields, and prints a side-by-side summary. If JSON parsing fails or fields
are missing, the error is caught and reported instead of crashing.
