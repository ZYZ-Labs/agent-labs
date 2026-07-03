"""Test client for the minimal LSP server.

Starts the LSP server as a subprocess, sends initialize/didOpen/definition,
and prints the responses.
"""

import json
import subprocess
import sys
from pathlib import Path


SERVER_SCRIPT = Path(__file__).resolve().parent / "main.py"


def read_message(stream) -> dict[str, object] | None:
    headers: dict[str, str] = {}
    while True:
        line = stream.readline()
        if not line:
            return None
        line = line.decode("utf-8").strip()
        if not line:
            break
        key, value = line.split(":", 1)
        headers[key.strip().lower()] = value.strip()
    length = int(headers.get("content-length", 0))
    if length == 0:
        return None
    return json.loads(stream.read(length).decode("utf-8"))


def send_message(stream, message: dict[str, object]) -> None:
    body = json.dumps(message, ensure_ascii=False).encode("utf-8")
    stream.write(f"Content-Length: {len(body)}\r\n\r\n".encode("utf-8"))
    stream.write(body)
    stream.flush()


def main():
    sample_code = '\n'.join([
        'def add(a, b):',
        '    return a + b',
        '',
        'result = add(1, 2)',
    ])
    uri = "file:///tmp/test.py"

    proc = subprocess.Popen(
        [sys.executable, str(SERVER_SCRIPT)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=False,
        bufsize=0,
    )

    try:
        send_message(
            proc.stdin,
            {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize",
                "params": {"processId": None, "rootUri": None, "capabilities": {}},
            },
        )
        print("initialize =", read_message(proc.stdout))

        send_message(
            proc.stdin,
            {"jsonrpc": "2.0", "method": "initialized", "params": {}},
        )

        send_message(
            proc.stdin,
            {
                "jsonrpc": "2.0",
                "method": "textDocument/didOpen",
                "params": {
                    "textDocument": {
                        "uri": uri,
                        "languageId": "python",
                        "text": sample_code,
                    }
                },
            },
        )

        send_message(
            proc.stdin,
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "textDocument/definition",
                "params": {
                    "textDocument": {"uri": uri},
                    "position": {"line": 3, "character": 8},
                },
            },
        )
        print("definition =", read_message(proc.stdout))

        send_message(proc.stdin, {"jsonrpc": "2.0", "id": 3, "method": "shutdown"})
        print("shutdown =", read_message(proc.stdout))

        send_message(proc.stdin, {"jsonrpc": "2.0", "method": "exit"})
    finally:
        proc.stdin.close()
        proc.wait(timeout=2)


if __name__ == "__main__":
    main()
