# Lab 29: Deployment

## Objectives

- Package a simple agent service as a Docker image.
- Build a minimal FastAPI server exposing a `/chat` endpoint.
- Accept environment variables for credentials and model selection.
- Return graceful errors when configuration is missing.

## Run Locally

```bash
cd python
pip install -r requirements.txt
python main.py
```

Then POST to `http://localhost:8000/chat`:

```bash
curl -X POST http://localhost:8000/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"Hello, agent!"}'
```

## Build & Run with Docker

```bash
cd python
docker build -t agent-lab-29 .
docker run -p 8000:8000 -e OPENAI_API_KEY=$OPENAI_API_KEY agent-lab-29
```

## Expected Output

The server responds to `/chat` with JSON containing the assistant's reply, or a clear error if `OPENAI_API_KEY` is missing.
