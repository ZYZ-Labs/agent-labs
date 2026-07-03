# Lab 10: MCP Client over Stdio + SSE Transport Concept

## Objectives

- Build a minimal MCP client that spawns a server subprocess and talks JSON-RPC over stdio.
- Perform the `initialize` handshake, discover tools with `tools/list`, and invoke one with `tools/call`.
- Understand the difference between stdio and SSE transports by parsing SSE events.

## Run

First make sure the server from Lab 09 exists, then run the client:

```bash
cd python
pip install -r requirements.txt
python main.py
```

The client points to `../09-mcp-server-stdio/python/main.py` by default.
Set `MCP_SERVER_SCRIPT` to override the server path.

## Expected Output

The client prints the initialize result, the list of tools, and the result of
invoking `calculate("(10 + 5) / 3")`. It also prints a few parsed SSE events to
show the shape of Server-Sent Events.
