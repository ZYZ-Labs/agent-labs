# Lab 04: Tool Calling from Scratch

## Objectives

- Define tool schemas in the OpenAI format.
- Parse `tool_calls` from model responses.
- Execute tools and inject results back into the conversation.
- Implement a tiny agent that can call multiple tools in one session.

## Tools

- `get_weather(city: str)` — returns fake weather data.
- `search_notes(query: str)` — searches a small in-memory note database.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

The model decides which tools to call, the program executes them, and the model answers the user's question using the results.
