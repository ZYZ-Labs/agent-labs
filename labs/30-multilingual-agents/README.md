# Lab 30: Multilingual Agents

## Objectives

- Implement the same tiny agent in four languages: Python, Node.js, Go, and Java.
- Compare syntax, HTTP client usage, and environment-variable handling.
- Keep the Python version runnable; the others are complete code samples.

## Run

### Python

```bash
cd python
pip install -r requirements.txt
python main.py
```

### Node.js (sample)

```bash
cd node
npm install
node main.js
```

### Go (sample)

```bash
cd go
go run main.go
```

### Java (sample)

```bash
cd java
# Requires Maven or Gradle setup
./mvnw compile exec:java -Dexec.mainClass="AgentLab"
```

## Expected Output

All four versions send a chat request to the configured OpenAI-compatible endpoint and print the assistant's reply.
