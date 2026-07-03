"""Minimal MCP server over stdio (JSON-RPC with length-prefixed framing).

Implements:
  - initialize
  - notifications/initialized
  - tools/list
  - tools/call

Tools:
  - get_current_time(timezone: str = "UTC") -> ISO timestamp
  - calculate(expression: str) -> number
"""

import argparse
import ast
import json
import logging
import operator
import sys
from datetime import datetime, timezone as _timezone
from typing import Any

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    stream=sys.stderr,
)
logger = logging.getLogger("mcp-server")


# ---------- safe calculator ----------

_ALLOWED_BIN_OPS = {
    ast.Add: operator.add,
    ast.Sub: operator.sub,
    ast.Mult: operator.mul,
    ast.Div: operator.truediv,
    ast.Pow: operator.pow,
    ast.Mod: operator.mod,
}

_ALLOWED_UNARY_OPS = {
    ast.UAdd: operator.pos,
    ast.USub: operator.neg,
}


def safe_eval(node: ast.AST) -> float:
    if isinstance(node, ast.Expression):
        return safe_eval(node.body)
    if isinstance(node, ast.Constant) and isinstance(node.value, (int, float)):
        return float(node.value)
    if isinstance(node, ast.BinOp):
        op_type = type(node.op)
        if op_type not in _ALLOWED_BIN_OPS:
            raise ValueError(f"Unsupported binary operator: {op_type.__name__}")
        return _ALLOWED_BIN_OPS[op_type](safe_eval(node.left), safe_eval(node.right))
    if isinstance(node, ast.UnaryOp):
        op_type = type(node.op)
        if op_type not in _ALLOWED_UNARY_OPS:
            raise ValueError(f"Unsupported unary operator: {op_type.__name__}")
        return _ALLOWED_UNARY_OPS[op_type](safe_eval(node.operand))
    raise ValueError(f"Unsupported expression: {type(node).__name__}")


def calculate(expression: str) -> dict[str, Any]:
    try:
        tree = ast.parse(expression.strip(), mode="eval")
        value = safe_eval(tree)
        return {"content": [{"type": "text", "text": str(value)}], "isError": False}
    except Exception as exc:
        return {
            "content": [{"type": "text", "text": f"Invalid expression: {exc}"}],
            "isError": True,
        }


# ---------- tools ----------

TOOLS = [
    {
        "name": "get_current_time",
        "description": "Return the current time in ISO 8601 format.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "timezone": {
                    "type": "string",
                    "description": "IANA timezone name, e.g. UTC or Asia/Shanghai.",
                }
            },
        },
    },
    {
        "name": "calculate",
        "description": "Evaluate a simple arithmetic expression.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "expression": {
                    "type": "string",
                    "description": "Arithmetic expression with +, -, *, /, parentheses, numbers.",
                }
            },
            "required": ["expression"],
        },
    },
]


def get_current_time(timezone: str = "UTC") -> dict[str, Any]:
    try:
        import zoneinfo

        tz = zoneinfo.ZoneInfo(timezone)
    except Exception:
        tz = _timezone.utc if timezone.upper() == "UTC" else None
    if tz is None:
        return {
            "content": [
                {
                    "type": "text",
                    "text": f"Unsupported timezone: {timezone}. Using UTC.",
                }
            ],
            "isError": True,
        }
    now = datetime.now(tz)
    return {
        "content": [{"type": "text", "text": now.isoformat()}],
        "isError": False,
    }


TOOL_HANDLERS = {
    "get_current_time": get_current_time,
    "calculate": calculate,
}


# ---------- JSON-RPC framing ----------


def read_message(stream: Any) -> dict[str, Any] | None:
    """Read a length-prefixed JSON-RPC message from a binary stream."""
    headers = {}
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
    """Write a length-prefixed JSON-RPC message to a binary stream."""
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
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "agent-labs-mcp-server", "version": "0.1.0"},
            },
        )

    if method == "notifications/initialized":
        return None  # notification, no response

    if method == "tools/list":
        return make_response(req_id, {"tools": TOOLS})

    if method == "tools/call":
        name = params.get("name")
        arguments = params.get("arguments", {})
        handler = TOOL_HANDLERS.get(name)
        if not handler:
            return make_error(req_id, -32601, f"Unknown tool: {name}")
        try:
            result = handler(**arguments)
        except Exception as exc:
            return make_error(req_id, -32603, f"Tool error: {exc}")
        return make_response(req_id, result)

    return make_error(req_id, -32601, f"Method not found: {method}")


def serve(stdin=None, stdout=None) -> None:
    stdin = stdin or sys.stdin.buffer
    stdout = stdout or sys.stdout.buffer
    logger.info("MCP server ready on stdio")
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

        response = handle_request(request)
        if response is not None:
            write_message(stdout, response)


# ---------- smoke test ----------


def smoke_test():
    """Run a self-contained smoke test using an in-memory stdin/stdout pair."""
    import io

    requests = [
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {"protocolVersion": "2024-11-05", "capabilities": {}},
        },
        {"jsonrpc": "2.0", "id": None, "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
        {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "tools/call",
            "params": {"name": "get_current_time", "arguments": {"timezone": "UTC"}},
        },
        {
            "jsonrpc": "2.0",
            "id": 4,
            "method": "tools/call",
            "params": {"name": "calculate", "arguments": {"expression": "(2 + 3) * 4"}},
        },
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
    parser = argparse.ArgumentParser(description="Minimal MCP server over stdio")
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
