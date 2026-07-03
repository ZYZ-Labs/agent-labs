"""SRE agent: diagnose logs, suggest remediation, ask for confirmation.

Reads a log file, uses an LLM to diagnose, suggests remediation commands,
and asks for human confirmation before executing.
"""
import argparse
import json
import logging
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("sre-agent")

# Whitelist of command prefixes the agent is allowed to execute.
ALLOWED_COMMANDS = {
    "systemctl restart",
    "kubectl rollout restart",
    "docker restart",
    "echo",
    "df",
}


def parse_logs(path: Path) -> list[dict]:
    pattern = re.compile(r"^(?P<ts>\S+)\s+(?P<level>\w+)\s+(?P<message>.+)$")
    entries = []
    for line in path.read_text(encoding="utf-8").splitlines():
        match = pattern.match(line)
        if match:
            entries.append(match.groupdict())
        else:
            entries.append({"ts": "", "level": "UNKNOWN", "message": line})
    return entries


def summarize_errors(entries: list[dict]) -> dict:
    error_messages = [
        e["message"] for e in entries if e["level"] in ("ERROR", "FATAL")
    ]
    counts = {}
    for e in entries:
        counts[e["level"]] = counts.get(e["level"], 0) + 1
    return {"error_messages": error_messages, "counts": counts}


def diagnose(entries: list[dict], client: OpenAIClient | None) -> dict:
    if not client:
        return {
            "diagnosis": (
                "Database appears unreachable (multiple connection timeouts and "
                "503 errors). Disk usage is also elevated."
            ),
            "commands": [
                "echo 'Checking database service status...'",
                "systemctl restart postgresql",
                "echo 'Monitoring disk usage...'",
                "df -h",
            ],
            "risk": "medium",
        }

    log_text = "\n".join(f"{e['level']}: {e['message']}" for e in entries)
    prompt = f"""You are an SRE agent. Diagnose the following logs and respond with JSON in this exact shape:
{{
  "diagnosis": "short diagnosis",
  "commands": ["command1", "command2"],
  "risk": "low|medium|high"
}}

Logs:
{log_text}"""

    resp = client.chat_completion(
        messages=[{"role": "user", "content": prompt}],
        temperature=0.2,
        max_tokens=200,
        response_format={"type": "json_object"},
    )
    content = resp["choices"][0]["message"]["content"]
    try:
        parsed = json.loads(content)
    except json.JSONDecodeError:
        parsed = {"diagnosis": content, "commands": [], "risk": "unknown"}
    return parsed


def is_command_allowed(command: str) -> bool:
    for prefix in ALLOWED_COMMANDS:
        if command.strip().startswith(prefix):
            return True
    return False


def execute_commands(commands: list[str]) -> list[dict]:
    results = []
    for cmd in commands:
        print(f"  $ {cmd}")
        if not is_command_allowed(cmd):
            print("    -> SKIPPED (not in allowlist)")
            results.append({"command": cmd, "status": "skipped", "output": ""})
            continue
        try:
            result = subprocess.run(
                cmd,
                shell=True,
                capture_output=True,
                text=True,
                timeout=10,
                check=False,
            )
            output = (result.stdout + result.stderr).strip() or "<no output>"
            print(f"    -> exit {result.returncode}")
            results.append(
                {"command": cmd, "status": "executed", "output": output}
            )
        except Exception as exc:
            print(f"    -> ERROR: {exc}")
            results.append({"command": cmd, "status": "error", "output": str(exc)})
    return results


def main() -> None:
    configure_logging()
    parser = argparse.ArgumentParser(description="SRE agent")
    parser.add_argument(
        "logfile", nargs="?", default="sample_app.log", help="Log file to analyze"
    )
    args = parser.parse_args()

    client = None
    try:
        client = OpenAIClient()
    except ValueError as exc:
        logger.warning("LLM client disabled: %s", exc)

    log_path = Path(args.logfile)
    if not log_path.exists():
        logger.error("Log file not found: %s", log_path)
        sys.exit(1)

    entries = parse_logs(log_path)
    print(f"Read {len(entries)} log entries from {log_path}")
    diagnosis = diagnose(entries, client)

    print("\n=== Diagnosis ===")
    print(diagnosis["diagnosis"])
    print(f"Risk: {diagnosis.get('risk', 'unknown')}")

    commands = diagnosis.get("commands", [])
    print("\n=== Proposed remediation commands ===")
    for cmd in commands:
        print(f"  - {cmd}")

    if not commands:
        print("No remediation commands proposed.")
        return

    answer = input("\nExecute proposed commands? [y/n]: ").strip().lower()
    if answer not in ("y", "yes"):
        print("Execution cancelled by operator.")
        return

    print("\nExecuting commands...")
    results = execute_commands(commands)
    print("\nExecution summary:")
    print(json.dumps(results, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
