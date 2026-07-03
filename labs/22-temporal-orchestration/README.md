# Lab 22: Temporal Orchestration

## Objectives

- Understand Temporal workflow and activity primitives.
- Run a minimal Temporal worker and client in Python.
- Gracefully handle a missing Temporal server.

## Run

Start a local Temporal server with Docker:

```bash
cd python
docker compose up -d
```

Then start the worker in one terminal:

```bash
python worker.py
```

And run the client in another terminal:

```bash
python main.py
```

If Docker is unavailable, the scripts print a clear error and instructions.

## Expected Output

The client starts a `GreetingWorkflow`, the worker executes the `compose_greeting` activity, and the client prints:

```
Workflow result: Hello, Agent Engineer from Temporal!
```
