import os
import sys
from pathlib import Path

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

configure_logging()

app = FastAPI(title="Agent Lab 29: Deployment")

# Validate config at startup and fail gracefully.
api_key = os.getenv("OPENAI_API_KEY", "")
if not api_key:
    print("Warning: OPENAI_API_KEY is not set. /chat will return 503 until configured.", file=sys.stderr)

client = OpenAIClient()


class ChatRequest(BaseModel):
    message: str
    model: str | None = None


class ChatResponse(BaseModel):
    reply: str
    model: str


@app.get("/health")
def health() -> dict:
    return {"status": "ok", "configured": bool(api_key)}


@app.post("/chat", response_model=ChatResponse)
def chat(request: ChatRequest) -> ChatResponse:
    if not api_key:
        raise HTTPException(status_code=503, detail="OPENAI_API_KEY is not configured")

    if not request.message.strip():
        raise HTTPException(status_code=400, detail="message is required")

    try:
        response = client.chat_completion(
            messages=[{"role": "user", "content": request.message}],
            model=request.model,
            max_tokens=300,
            temperature=0.0,
        )
        reply = response["choices"][0]["message"].get("content", "")
        return ChatResponse(reply=reply, model=request.model or client.model)
    except Exception as exc:
        raise HTTPException(status_code=502, detail=f"Upstream error: {exc}") from exc


def main():
    import uvicorn

    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=False)


if __name__ == "__main__":
    main()
