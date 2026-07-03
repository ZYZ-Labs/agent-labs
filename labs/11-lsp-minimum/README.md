# Lab 11: Minimal LSP Server over Stdio

## Objectives

- Implement a tiny [Language Server Protocol](https://microsoft.github.io/language-server-protocol/) server over stdio.
- Handle the lifecycle: `initialize`, `initialized`, `shutdown`, `exit`.
- Handle document open (`textDocument/didOpen`) and a single navigation request (`textDocument/definition`).
- Include a small test client that drives the server in a subprocess.

## Run

### Server mode

```bash
cd python
python main.py
```

The server reads JSON-RPC messages on stdin and writes responses on stdout.

### Built-in smoke test

```bash
cd python
python main.py --smoke
```

### External test client

```bash
cd python
python main.py &   # start server in background
python test_client.py
```

## Expected Output

The smoke test opens a Python file and asks for the definition of `greet`.
The server returns a `Location` pointing to the line where `def greet` is defined.
