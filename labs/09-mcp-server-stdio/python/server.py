"""Minimal MCP server speaking JSON-RPC 2.0 over stdio.

Implements:
- initialize
- initialized (notification)
- tools/list
- tools/call

Transport: length-prefixed messages with Content-Length header.
"""

import json
import logging
import re
import sys
from datetime import datetime, timezone

logger = logging.getLogger("mcp-server")

PROTOCOL_VERSION = "2024-11-05"
SERVER_NAME = "agentlabs-minimal-mcp-server"
SERVER_VERSION = "0.1.0"

TOOLS = [
    {
        "name": "get_current_time",
        "description": "Return the current UTC time.",
        "inputSchema": {
            "type": "object",
            "properties": {},
        },
    },
    {
        "name": "calculate",
        "description": "Evaluate a simple arithmetic expression with +, -, *, /, **, and parentheses.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "expression": {
                    "type": "string",
                    "description": "Arithmetic expression to evaluate, e.g. '2 + 3 * 4'.",
                }
            },
            "required": ["expression"],
        },
    },
]


def send_message(obj: dict) -> None:
    """Send a length-prefixed JSON-RPC message on stdout."""
    body = json.dumps(obj, ensure_ascii=False)
    payload = body.encode("utf-8")
    header = f"Content-Length: {len(payload)}\r\n\r\n".encode("ascii")
    sys.stdout.buffer.write(header + payload)
    sys.stdout.buffer.flush()


def make_result(id_, result: dict) -> dict:
    return {"jsonrpc": "2.0", "id": id_, "result": result}


def make_error(id_, code: int, message: str, data=None) -> dict:
    err = {"code": code, "message": message}
    if data is not None:
        err["data"] = data
    return {"jsonrpc": "2.0", "id": id_, "error": err}


def safe_calculate(expression: str) -> str:
    """Evaluate a restricted arithmetic expression."""
    allowed = set("0123456789+-*/().** ")
    if not expression or not set(expression).issubset(allowed):
        raise ValueError("Expression contains disallowed characters.")
    # Disallow consecutive operators that could be malicious; keep it simple.
    if "__" in expression:
        raise ValueError("Expression contains disallowed pattern.")
    try:
        result = eval(expression, {"__builtins__": {}}, {})
    except Exception as exc:
        raise ValueError(f"Invalid expression: {exc}") from exc
    return str(result)


def handle_request(request: dict) -> dict | None:
    method = request.get("method")
    params = request.get("params", {})
    req_id = request.get("id")

    logger.info("Received request: %s", method)

    if method == "initialize":
        return make_result(
            req_id,
            {
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {"tools": {}},
                "serverInfo": {"name": SERVER_NAME, "version": SERVER_VERSION},
            },
        )

    if method == "initialized":
        # Notification: no response.
        logger.info("Handshake completed.")
        return None

    if method == "tools/list":
        return make_result(req_id, {"tools": TOOLS})

    if method == "tools/call":
        name = params.get("name")
        arguments = params.get("arguments", {})
        if name == "get_current_time":
            now = datetime.now(timezone.utc).isoformat()
            return make_result(
                req_id,
                {
                    "content": [
                        {"type": "text", "text": now},
                    ],
                    "isError": False,
                },
            )
        if name == "calculate":
            expression = arguments.get("expression", "")
            try:
                value = safe_calculate(expression)
                return make_result(
                    req_id,
                    {
                        "content": [
                            {"type": "text", "text": value},
                        ],
                        "isError": False,
                    },
                )
            except ValueError as exc:
                return make_result(
                    req_id,
                    {
                        "content": [
                            {"type": "text", "text": str(exc)},
                        ],
                        "isError": True,
                    },
                )
        return make_error(req_id, -32601, f"Unknown tool: {name}")

    return make_error(req_id, -32601, f"Method not found: {method}")


def read_messages():
    """Yield parsed JSON-RPC messages from stdin using Content-Length framing."""
    while True:
        header = b""
        while True:
            byte = sys.stdin.buffer.read(1)
            if not byte:
                return
            header += byte
            if header.endswith(b"\r\n\r\n"):
                break

        match = re.search(rb"Content-Length:\s*(\d+)", header, re.IGNORECASE)
        if not match:
            logger.error("Missing Content-Length header: %s", header)
            continue
        length = int(match.group(1))
        body = sys.stdin.buffer.read(length)
        try:
            yield json.loads(body.decode("utf-8"))
        except json.JSONDecodeError as exc:
            logger.error("Failed to decode JSON body: %s", exc)


def main():
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
        stream=sys.stderr,
    )
    logger.info("Starting %s v%s", SERVER_NAME, SERVER_VERSION)

    for request in read_messages():
        if not isinstance(request, dict):
            send_message(make_error(None, -32700, "Parse error"))
            continue
        response = handle_request(request)
        if response is not None:
            send_message(response)


if __name__ == "__main__":
    main()
