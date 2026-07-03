"""Example MCP client that talks to the stdio server from server.py."""

import json
import subprocess
import sys
from pathlib import Path


def encode_message(obj: dict) -> bytes:
    body = json.dumps(obj, ensure_ascii=False).encode("utf-8")
    header = f"Content-Length: {len(body)}\r\n\r\n".encode("ascii")
    return header + body


def read_message(stream) -> dict:
    header = b""
    while True:
        byte = stream.read(1)
        if not byte:
            raise EOFError("Server closed stdout while reading header.")
        header += byte
        if header.endswith(b"\r\n\r\n"):
            break

    length = int(header.split(b":")[1].strip())
    body = stream.read(length)
    return json.loads(body.decode("utf-8"))


def send_request(proc: subprocess.Popen, method: str, params: dict, req_id: int) -> dict:
    request = {
        "jsonrpc": "2.0",
        "id": req_id,
        "method": method,
        "params": params,
    }
    proc.stdin.write(encode_message(request))
    proc.stdin.flush()
    return read_message(proc.stdout)


def send_notification(proc: subprocess.Popen, method: str, params: dict) -> None:
    request = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
    }
    proc.stdin.write(encode_message(request))
    proc.stdin.flush()


def main():
    server_script = Path(__file__).with_name("server.py")
    proc = subprocess.Popen(
        [sys.executable, str(server_script)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        bufsize=0,
    )

    try:
        # Handshake
        init_response = send_request(
            proc,
            "initialize",
            {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "agentlabs-mcp-client", "version": "0.1.0"},
            },
            req_id=1,
        )
        print("[initialize] server info:", init_response.get("result", {}).get("serverInfo"))

        send_notification(proc, "initialized", {})
        print("[initialized] handshake complete")

        # List tools
        tools_response = send_request(proc, "tools/list", {}, req_id=2)
        tools = tools_response.get("result", {}).get("tools", [])
        print("\n[tools/list] Available tools:")
        for tool in tools:
            print(f"  - {tool['name']}: {tool['description']}")

        # Call calculate
        call_response = send_request(
            proc,
            "tools/call",
            {"name": "calculate", "arguments": {"expression": "2 + 3 * 4"}},
            req_id=3,
        )
        print("\n[tools/call] calculate(2 + 3 * 4):")
        for item in call_response.get("result", {}).get("content", []):
            print(f"  {item.get('type')}: {item.get('text')}")

        # Call get_current_time
        call_response = send_request(
            proc,
            "tools/call",
            {"name": "get_current_time", "arguments": {}},
            req_id=4,
        )
        print("\n[tools/call] get_current_time():")
        for item in call_response.get("result", {}).get("content", []):
            print(f"  {item.get('type')}: {item.get('text')}")

    finally:
        proc.stdin.close()
        proc.wait(timeout=5)


if __name__ == "__main__":
    main()
