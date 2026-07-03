"""Human-in-the-loop workflow.

Pauses for human approval before executing a destructive action and supports
resuming from persisted state.
"""
import argparse
import json
import logging
import sys
import time
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import configure_logging

logger = logging.getLogger("human-in-the-loop")

STATE_FILE = Path(__file__).with_suffix("").parent / "workflow_state.json"


class DestructiveAction:
    def __init__(self, target: str, action: str) -> None:
        self.target = target
        self.action = action

    def describe(self) -> str:
        return f"{self.action} {self.target}"


class HumanInTheLoopWorkflow:
    def __init__(self, state_file: Path) -> None:
        self.state_file = state_file
        self.state = self._load()

    def _load(self) -> dict[str, Any]:
        if self.state_file.exists():
            with open(self.state_file, "r", encoding="utf-8") as f:
                return json.load(f)
        return {}

    def _save(self) -> None:
        tmp = self.state_file.with_suffix(".tmp")
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(self.state, f, indent=2)
        tmp.replace(self.state_file)

    def reset(self) -> None:
        if self.state_file.exists():
            self.state_file.unlink()
        self.state = {}

    def request_approval(self, action: DestructiveAction) -> None:
        if self.state.get("stage") in ("approved", "executed", "rejected"):
            return
        print(f"\nDestructive action requested: {action.describe()}")
        print("This action cannot be undone. Please review carefully.")
        self.state = {
            "stage": "awaiting_approval",
            "target": action.target,
            "action": action.action,
            "approved": None,
        }
        self._save()

    def collect_decision(self) -> bool | None:
        if self.state.get("stage") != "awaiting_approval":
            return self.state.get("approved")

        while True:
            answer = input("Approve? [y/n]: ").strip().lower()
            if answer in ("y", "yes"):
                self.state["approved"] = True
                self.state["stage"] = "approved"
                self._save()
                return True
            if answer in ("n", "no"):
                self.state["approved"] = False
                self.state["stage"] = "rejected"
                self._save()
                return False
            print("Please answer 'y' or 'n'.")

    def execute(self, action: DestructiveAction) -> None:
        if self.state.get("stage") == "executed":
            print("Action already executed.")
            return
        if not self.state.get("approved"):
            print("Action not approved; will not execute.")
            return
        print(f"\nExecuting: {action.describe()}")
        # Simulate destructive work.
        print(f"  -> {action.target} has been {action.action}d.")
        self.state["stage"] = "executed"
        self.state["executed_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
        self._save()

    def run(self, action: DestructiveAction) -> None:
        self.request_approval(action)
        approved = self.collect_decision()
        if approved:
            self.execute(action)
        else:
            print("\nAction rejected by human operator.")


def main() -> None:
    configure_logging()
    parser = argparse.ArgumentParser(description="Human-in-the-loop destructive action")
    parser.add_argument("--reset", action="store_true", help="Clear saved workflow state")
    args = parser.parse_args()

    workflow = HumanInTheLoopWorkflow(STATE_FILE)
    if args.reset:
        workflow.reset()
        print("Workflow state reset.")
        return

    action = DestructiveAction(target="user_account_42", action="delete")
    workflow.run(action)


if __name__ == "__main__":
    main()
