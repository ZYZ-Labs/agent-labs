# Lab 09: Handwritten MCP Server over Stdio

## Objectives

- Implement a minimal [Model Context Protocol](https://modelcontextprotocol.io/) server from scratch.
- Speak JSON-RPC over standard input/output with length-prefixed messages.
- Handle the `initialize` handshake, `tools/list`, and `tools/call` methods.
- Register and execute two simple tools: `get_current_time` and `calculate`.

## Run

Start the server and type a raw JSON-RPC request, or run the built-in smoke test:

```bash
cd python
python main.py --smoke
```

For interactive use, run the server without arguments and send MCP messages on stdin.

## Expected Output

The smoke test sends `initialize`, `tools/list`, and `tools/call` for both tools,
then prints the responses. The server stays alive until it sees `exit` or EOF.
