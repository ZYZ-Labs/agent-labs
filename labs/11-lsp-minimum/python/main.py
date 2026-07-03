"""Minimal LSP server over stdio (JSON-RPC with Content-Length framing).

Implements:
  - initialize
  - initialized (notification)
  - textDocument/didOpen
  - textDocument/definition
  - shutdown
  - exit
"""

import argparse
import json
import logging
import re
import sys
from typing import Any

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    stream=sys.stderr,
)
logger = logging.getLogger("lsp-server")


# ---------- document store ----------

DOCUMENTS: dict[str, str] = {}


def update_document(uri: str, text: str) -> None:
    DOCUMENTS[uri] = text


def find_definition(uri: str, position: dict[str, int]) -> dict[str, Any] | None:
    """Return a Location if the word under the cursor is a function name defined somewhere."""
    text = DOCUMENTS.get(uri, "")
    lines = text.splitlines()
    line_idx = position.get("line", 0)
    char_idx = position.get("character", 0)
    if not (0 <= line_idx < len(lines)):
        return None
    line = lines[line_idx]
    match = re.search(r"[A-Za-z0-9_]+", line[char_idx:] if char_idx < len(line) else "")
    if not match:
        return None
    word = match.group(0)

    # Search all open documents for "def word("
    pattern = re.compile(rf"^def\s+{re.escape(word)}\s*\(", re.MULTILINE)
    for doc_uri, doc_text in DOCUMENTS.items():
        for m in pattern.finditer(doc_text):
            start_line = doc_text[: m.start()].count("\n")
            return {
                "uri": doc_uri,
                "range": {
                    "start": {"line": start_line, "character": 0},
                    "end": {"line": start_line, "character": len(f"def {word}(")},
                },
            }
    return None


# ---------- JSON-RPC framing ----------


def read_message(stream: Any) -> dict[str, Any] | None:
    headers: dict[str, str] = {}
    while True:
        line = stream.readline()
        if not line:
            return None
        line = line.decode("utf-8").strip()
        if not line:
            break
        if ":" in line:
            key, value = line.split(":", 1)
            headers[key.strip().lower()] = value.strip()

    length = int(headers.get("content-length", 0))
    if length == 0:
        return None
    body = stream.read(length)
    return json.loads(body.decode("utf-8"))


def write_message(stream: Any, message: dict[str, Any]) -> None:
    body = json.dumps(message, ensure_ascii=False).encode("utf-8")
    stream.write(f"Content-Length: {len(body)}\r\n\r\n".encode("utf-8"))
    stream.write(body)
    stream.flush()


# ---------- request handlers ----------


def make_response(request_id: Any, result: Any) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "result": result}


def make_error(request_id: Any, code: int, message: str) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}


def handle_request(request: dict[str, Any]) -> dict[str, Any] | None:
    method = request.get("method")
    params = request.get("params", {})
    req_id = request.get("id")
    logger.info("Received %s", method)

    if method == "initialize":
        return make_response(
            req_id,
            {
                "capabilities": {
                    "textDocumentSync": {"openClose": True, "change": 0},
                    "definitionProvider": True,
                },
                "serverInfo": {"name": "agent-labs-lsp", "version": "0.1.0"},
            },
        )

    if method == "initialized":
        return None

    if method == "textDocument/didOpen":
        doc = params.get("textDocument", {})
        update_document(doc.get("uri", ""), doc.get("text", ""))
        return None

    if method == "textDocument/definition":
        td = params.get("textDocument", {})
        pos = params.get("position", {})
        location = find_definition(td.get("uri", ""), pos)
        return make_response(req_id, location)

    if method == "shutdown":
        return make_response(req_id, None)

    if method == "exit":
        return None

    return make_error(req_id, -32601, f"Method not found: {method}")


def serve(stdin=None, stdout=None) -> None:
    stdin = stdin or sys.stdin.buffer
    stdout = stdout or sys.stdout.buffer
    logger.info("LSP server ready on stdio")
    while True:
        try:
            request = read_message(stdin)
        except json.JSONDecodeError as exc:
            logger.error("Bad JSON: %s", exc)
            write_message(stdout, make_error(None, -32700, "Parse error"))
            continue
        if request is None:
            logger.info("EOF reached; shutting down")
            break

        method = request.get("method")
        response = handle_request(request)
        if response is not None:
            write_message(stdout, response)
        if method == "exit":
            break


# ---------- smoke test ----------


def smoke_test():
    import io

    sample_code = '\n'.join([
        'def greet(name):',
        '    return f"Hello, {name}!"',
        '',
        'print(greet("world"))',
    ])
    uri = "file:///tmp/sample.py"

    requests = [
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {"processId": None, "rootUri": None, "capabilities": {}},
        },
        {"jsonrpc": "2.0", "method": "initialized", "params": {}},
        {
            "jsonrpc": "2.0",
            "method": "textDocument/didOpen",
            "params": {"textDocument": {"uri": uri, "languageId": "python", "text": sample_code}},
        },
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "textDocument/definition",
            "params": {
                "textDocument": {"uri": uri},
                "position": {"line": 3, "character": 6},  # cursor on greet
            },
        },
        {"jsonrpc": "2.0", "id": 3, "method": "shutdown"},
        {"jsonrpc": "2.0", "method": "exit"},
    ]

    stdin = io.BytesIO()
    for req in requests:
        body = json.dumps(req).encode("utf-8")
        stdin.write(f"Content-Length: {len(body)}\r\n\r\n".encode("utf-8"))
        stdin.write(body)
    stdin.seek(0)

    stdout = io.BytesIO()
    serve(stdin, stdout)
    stdout.seek(0)
    print("Smoke test output:")
    print(stdout.read().decode("utf-8"))


def main():
    parser = argparse.ArgumentParser(description="Minimal LSP server over stdio")
    parser.add_argument(
        "--smoke",
        action="store_true",
        help="Run a self-contained smoke test instead of serving.",
    )
    args = parser.parse_args()
    if args.smoke:
        smoke_test()
    else:
        serve()


if __name__ == "__main__":
    main()
