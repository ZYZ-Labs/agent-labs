"""Minimal MCP client over stdio, plus an SSE parsing demonstration.

Spawns the server from Lab 09 as a subprocess, performs the initialize
handshake, lists tools, calls one, then demonstrates how SSE events are parsed.
"""

import json
import logging
import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Iterator

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import configure_logging

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("mcp-client")


def _default_server_script() -> Path:
    return Path(__file__).resolve().parents[2] / "09-mcp-server-stdio" / "python" / "main.py"


class MCPStdioClient:
    """A tiny MCP client that drives a server subprocess over stdio."""

    def __init__(self, server_script: str | Path | None = None) -> None:
        script = Path(server_script or os.getenv("MCP_SERVER_SCRIPT") or _default_server_script())
        if not script.exists():
            raise FileNotFoundError(f"MCP server script not found: {script}")
        self.script = script
        self.process: subprocess.Popen | None = None
        self._request_id = 0

    def _next_id(self) -> int:
        self._request_id += 1
        return self._request_id

    def connect(self) -> None:
        logger.info("Starting MCP server: %s", self.script)
        self.process = subprocess.Popen(
            [sys.executable, str(self.script)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=False,
            bufsize=0,
        )

    def disconnect(self) -> None:
        if self.process and self.process.poll() is None:
            self.process.stdin.close()
            self.process.terminate()
            try:
                self.process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                self.process.kill()
        self.process = None

    def _send(self, message: dict[str, Any]) -> None:
        body = json.dumps(message, ensure_ascii=False).encode("utf-8")
        data = f"Content-Length: {len(body)}\r\n\r\n".encode("utf-8") + body
        self.process.stdin.write(data)
        self.process.stdin.flush()

    def _recv(self) -> dict[str, Any]:
        headers: dict[str, str] = {}
        while True:
            line = self.process.stdout.readline()
            if not line:
                raise ConnectionError("Server closed stdout")
            line = line.decode("utf-8").strip()
            if not line:
                break
            if ":" in line:
                key, value = line.split(":", 1)
                headers[key.strip().lower()] = value.strip()

        length = int(headers.get("content-length", 0))
        if length == 0:
            raise ConnectionError("Empty message body")
        body = self.process.stdout.read(length)
        return json.loads(body.decode("utf-8"))

    def initialize(self) -> dict[str, Any]:
        self._send(
            {
                "jsonrpc": "2.0",
                "id": self._next_id(),
                "method": "initialize",
                "params": {"protocolVersion": "2024-11-05", "capabilities": {}},
            }
        )
        result = self._recv()
        self._send(
            {"jsonrpc": "2.0", "id": None, "method": "notifications/initialized"}
        )
        return result

    def list_tools(self) -> list[dict[str, Any]]:
        self._send({"jsonrpc": "2.0", "id": self._next_id(), "method": "tools/list"})
        response = self._recv()
        if "error" in response:
            raise RuntimeError(response["error"])
        return response.get("result", {}).get("tools", [])

    def call_tool(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        self._send(
            {
                "jsonrpc": "2.0",
                "id": self._next_id(),
                "method": "tools/call",
                "params": {"name": name, "arguments": arguments},
            }
        )
        response = self._recv()
        if "error" in response:
            raise RuntimeError(response["error"])
        return response.get("result", {})


def parse_sse(lines: Iterator[bytes]) -> Iterator[dict[str, str]]:
    """Parse Server-Sent Events from an iterable of byte lines.

    Example SSE stream:
        event: message
        data: {"id": 1, "text": "hello"}

        event: done
        data: EOF
    """
    event: dict[str, str] = {}
    for raw in lines:
        line = raw.decode("utf-8").rstrip("\n").rstrip("\r")
        if not line:
            if event:
                yield event
                event = {}
            continue
        if line.startswith(":"):
            continue  # comment
        if ":" in line:
            key, value = line.split(":", 1)
            value = value.lstrip()
            event[key] = value
    if event:
        yield event


def demo_sse() -> None:
    """Demonstrate SSE parsing with a fake byte stream."""
    raw_stream = (
        b":heartbeat\n\n"
        b"event: message\n"
        b"data: {\"tool\": \"calculate\", \"args\": {\"expression\": \"1+1\"}}\n\n"
        b"event: status\n"
        b"data: processing\n\n"
        b"event: done\n"
        b"data: finished\n\n"
    )
    print("\n[SSE transport concept]")
    for event in parse_sse(iter(raw_stream.splitlines(keepends=True))):
        print("  SSE event:", event)


def main():
    configure_logging()
    client = MCPStdioClient()
    try:
        client.connect()
        init_response = client.initialize()
        print("[initialize]", json.dumps(init_response.get("result", {}), indent=2))

        tools = client.list_tools()
        print("\n[tools]")
        for tool in tools:
            print(f"  - {tool['name']}: {tool.get('description', '')}")

        result = client.call_tool("calculate", {"expression": "(10 + 5) / 3"})
        print("\n[tools/call calculate]")
        for item in result.get("content", []):
            print(" ", item.get("text"))

        demo_sse()
    except Exception as exc:
        logger.error("Client failed: %s", exc)
        raise
    finally:
        client.disconnect()


if __name__ == "__main__":
    main()
