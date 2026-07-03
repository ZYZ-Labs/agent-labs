"""Minimal A2A-style Agent Card, Task submission, and status polling.

Runs a small HTTP agent server and a client that discovers it, submits a task,
and polls the task status until completion.
"""

import json
import logging
import os
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("a2a-agent")

AGENT_CARD = {
    "name": "agent-labs-echo-agent",
    "description": "A minimal A2A agent that echoes input after a short delay.",
    "url": "http://localhost:8123",
    "version": "0.1.0",
    "capabilities": {
        "streaming": False,
        "pushNotifications": False,
    },
    "skills": [
        {
            "id": "echo",
            "name": "Echo",
            "description": "Returns the input text as the task result.",
        }
    ],
}

TASKS: dict[str, dict[str, Any]] = {}
TASK_LOCK = threading.Lock()


def create_task(message: dict[str, Any]) -> dict[str, Any]:
    task_id = str(uuid.uuid4())
    task = {
        "id": task_id,
        "status": {"state": "submitted", "timestamp": time.time()},
        "messages": [message],
        "artifacts": [],
    }
    with TASK_LOCK:
        TASKS[task_id] = task

    # Simulate async work in a background thread.
    def worker():
        time.sleep(2)
        with TASK_LOCK:
            task["status"] = {"state": "working", "timestamp": time.time()}
        time.sleep(2)
        with TASK_LOCK:
            task["status"] = {
                "state": "completed",
                "timestamp": time.time(),
                "message": {
                    "role": "agent",
                    "parts": [{"type": "text", "text": f"Echo: {message.get('parts', [{}])[0].get('text', '')}"}],
                },
            }

    threading.Thread(target=worker, daemon=True).start()
    return task


class AgentHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        logger.info(format % args)

    def _send_json(self, status: int, payload: Any) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/.well-known/agent.json":
            self._send_json(200, AGENT_CARD)
            return

        if parsed.path.startswith("/tasks/"):
            task_id = parsed.path.split("/")[-1]
            with TASK_LOCK:
                task = TASKS.get(task_id)
            if task:
                self._send_json(200, task)
            else:
                self._send_json(404, {"error": "Task not found"})
            return

        self._send_json(404, {"error": "Not found"})

    def do_POST(self):
        parsed = urlparse(self.path)
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length).decode("utf-8")
        try:
            payload = json.loads(body) if body else {}
        except json.JSONDecodeError:
            self._send_json(400, {"error": "Invalid JSON"})
            return

        if parsed.path == "/tasks/send":
            message = payload.get("message", {})
            task = create_task(message)
            self._send_json(200, task)
            return

        self._send_json(404, {"error": "Not found"})


def start_server(host: str = "localhost", port: int = 8123) -> HTTPServer:
    server = HTTPServer((host, port), AgentHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    logger.info("Agent server listening on http://%s:%d", host, port)
    return server


def fetch_agent_card(base_url: str) -> dict[str, Any]:
    import urllib.request

    with urllib.request.urlopen(f"{base_url}/.well-known/agent.json") as resp:
        return json.loads(resp.read().decode("utf-8"))


def submit_task(base_url: str, text: str) -> dict[str, Any]:
    import urllib.request

    payload = {
        "message": {
            "role": "user",
            "parts": [{"type": "text", "text": text}],
        }
    }
    req = urllib.request.Request(
        f"{base_url}/tasks/send",
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))


def get_task(base_url: str, task_id: str) -> dict[str, Any]:
    import urllib.request

    with urllib.request.urlopen(f"{base_url}/tasks/{task_id}") as resp:
        return json.loads(resp.read().decode("utf-8"))


def poll_task(base_url: str, task_id: str, timeout: float = 30.0, interval: float = 0.5) -> dict[str, Any]:
    deadline = time.time() + timeout
    while time.time() < deadline:
        task = get_task(base_url, task_id)
        state = task.get("status", {}).get("state")
        logger.info("Task %s state: %s", task_id[:8], state)
        if state in ("completed", "failed"):
            return task
        time.sleep(interval)
    raise TimeoutError(f"Task {task_id} did not complete within {timeout}s")


def main():
    base_url = os.getenv("A2A_AGENT_URL", "http://localhost:8123").rstrip("/")
    own_server = None

    try:
        # If the configured agent is not reachable, start one locally.
        try:
            fetch_agent_card(base_url)
            logger.info("Using existing agent at %s", base_url)
        except Exception:
            logger.info("No agent found at %s; starting local agent", base_url)
            own_server = start_server()
            # Wait briefly for the server to be ready.
            for _ in range(20):
                try:
                    fetch_agent_card(base_url)
                    break
                except Exception:
                    time.sleep(0.1)

        card = fetch_agent_card(base_url)
        print("\n[Agent Card]")
        print(json.dumps(card, indent=2, ensure_ascii=False))

        task = submit_task(base_url, "Hello from the A2A client!")
        print("\n[Submitted Task]")
        print(json.dumps(task, indent=2, ensure_ascii=False))

        final = poll_task(base_url, task["id"])
        print("\n[Final Task]")
        print(json.dumps(final, indent=2, ensure_ascii=False))

    finally:
        if own_server:
            own_server.shutdown()


if __name__ == "__main__":
    main()
