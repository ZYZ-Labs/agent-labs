# Lab 21: LangGraph Practice

## Objectives

- Use LangGraph to build a simple state graph.
- Nodes: retrieve context, decide action, (optionally) search more, generate answer.
- Route conditionally based on the LLM decision.
- Run the compiled graph end-to-end.

## Run

```bash
cd python
pip install -r requirements.txt
python main.py
```

Set `OPENAI_API_KEY` to use the LLM for routing and answer generation.

## Expected Output

The graph prints the chosen action, the retrieved (and possibly expanded) context, and the final answer.
