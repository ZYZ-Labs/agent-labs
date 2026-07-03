# AI Agent Engineering Labs

Companion executable code for the **AI Agent Engineering** course on [AI Engineer Toolbox](https://github.com/ZYZ-Labs/ai_engineer_toolbox).

This repository contains runnable, multi-language examples for backend engineers learning AI Agent development. Examples progress from a raw LLM call to production-grade orchestration, security, and observability.

## Repository Structure

```txt
labs/
  01-first-llm-call/        First OpenAI-compatible API call
  02-llm-parameters/        Complete parameter exploration
  03-react-loop/            Handwritten ReAct agent
  04-tool-calling/          Tool calling from scratch
  05-memory-system/         Context and vector memory
  06-planning-recovery/     Planning and self-recovery
  07-agent-evaluation/      Evaluating agent behavior
  08-function-json-mode/    Function calling vs JSON mode
  09-mcp-server-stdio/      Handwritten MCP server
  10-mcp-client/            MCP client with transports
  11-lsp-minimum/           Minimal LSP server
  12-lsp-treesitter/        LSP + Tree-sitter for agents
  13-a2a-agent/             Agent-to-Agent protocol example
  ...
  31-capstone-project/      Course capstone
shared/
  config/                   Common client wrappers and env example
  prompts/                  Reusable prompt templates
  fixtures/                 Sample data and test inputs
```

Each lab includes implementations in:

- **Python** (primary, complete)
- **Node / TypeScript**
- **Go**
- **Java**

## Quick Start

1. Copy the environment template:

```bash
cp shared/config/.env.example .env
# Edit .env with your API key or local endpoint
```

2. Choose a lab and language, install dependencies, and run:

```bash
# Python
cd labs/01-first-llm-call/python
pip install -r requirements.txt
python main.py

# Node
cd labs/01-first-llm-call/node
npm install
npx ts-node main.ts

# Go
cd labs/01-first-llm-call/go
go run main.go

# Java
cd labs/01-first-llm-call/java
mvn compile exec:java -Dexec.mainClass="Main"
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_API_KEY` | API key for OpenAI or compatible provider | (required unless using local) |
| `OPENAI_BASE_URL` | Base URL for the API | `https://api.openai.com/v1` |
| `OPENAI_MODEL` | Model name | `gpt-4o-mini` |
| `LOG_LEVEL` | Logging level | `INFO` |

Local alternatives (Ollama, etc.) are noted in individual lab READMEs.

## Docker Services

Some labs use external services (vector database, Temporal, Ollama). Start them with:

```bash
docker compose up -d
```

## License

Apache-2.0
