# Lab 13: Minimal A2A-Style Agent

## Objectives

- Implement a tiny server that exposes an [A2A](https://a2a.google.dev/)-style Agent Card.
- Support Task submission (`POST /tasks/send`) and Task status polling (`GET /tasks/{id}`).
- Write a client that discovers the agent, submits a task, and polls until completion.

## Run

The lab starts a local agent server, submits a task from a built-in client, and polls
for the result.

```bash
cd python
python main.py
```

Set `A2A_AGENT_URL` to point at an already-running agent; otherwise the script starts
one on `http://localhost:8123`.

## Expected Output

The client prints the Agent Card, the created Task object, and each polled status
until the task reaches `completed` (or `failed`).
