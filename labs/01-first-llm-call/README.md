# Lab 01: First LLM Call

## Objectives

- Configure an OpenAI-compatible API client from environment variables.
- Make your first chat completion request.
- Inspect the full response structure: choices, message, usage, finish_reason.
- Handle missing credentials and network errors gracefully.

## Files

- `python/main.py` — Python example using the shared `openai_client.py` wrapper.
- `node/main.ts` — TypeScript example using the shared `openai_client.ts` wrapper.
- `go/main.go` — Go example using the shared `openai_client.go` wrapper.
- `java/Main.java` — Java example using the shared `OpenAiClient.java` wrapper.

## Run

```bash
cp ../../../shared/config/.env.example .env
# Edit .env with your OPENAI_API_KEY
```

### Python

```bash
cd python
pip install -r requirements.txt
python main.py
```

### Node

```bash
cd node
npm install
npx ts-node main.ts
```

### Go

```bash
cd go
go run main.go
```

### Java

```bash
cd java
mvn compile exec:java -Dexec.mainClass="Main"
```

## Expected Output

You should see the assistant's reply, the model name, token usage, and the finish reason.
