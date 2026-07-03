import hashlib
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging


try:
    import chromadb
except ImportError:
    chromadb = None


class ShortTermMemory:
    def __init__(self, max_messages: int = 10):
        self.messages = []
        self.max_messages = max_messages

    def add(self, role: str, content: str) -> None:
        self.messages.append({"role": role, "content": content})
        if len(self.messages) > self.max_messages:
            # Keep system message if present, otherwise trim oldest
            keep = 1 if self.messages and self.messages[0]["role"] == "system" else 0
            self.messages = self.messages[:keep] + self.messages[keep + 1 :]

    def get(self) -> list:
        return list(self.messages)


class LongTermMemory:
    def __init__(self, client: OpenAIClient, collection_name: str = "agent_memory"):
        self.client = client
        self.collection = None
        if chromadb:
            db = chromadb.HttpClient(host="localhost", port=8000)
            self.collection = db.get_or_create_collection(collection_name)

    def embed(self, text: str) -> list[float]:
        # Use the chat model endpoint as a cheap proxy; replace with real embedding endpoint in production.
        resp = self.client.chat_completion(
            messages=[{"role": "user", "content": f"Summarize in one sentence for retrieval: {text}"}],
            max_tokens=20,
        )
        # In production use an embeddings endpoint. Here we hash to a fixed-size vector for demo compatibility.
        summary = resp["choices"][0]["message"]["content"]
        vec = [0.0] * 64
        for i, byte in enumerate(hashlib.sha256(summary.encode()).digest()):
            vec[i % 64] += byte / 255.0
        return vec

    def store(self, text: str, metadata: dict | None = None) -> None:
        if not self.collection:
            print("[Long-term memory] Chroma not available, skipping store.")
            return
        doc_id = hashlib.sha256(text.encode()).hexdigest()[:16]
        self.collection.add(
            ids=[doc_id],
            documents=[text],
            metadatas=[metadata or {}],
            embeddings=[self.embed(text)],
        )

    def retrieve(self, query: str, n: int = 3) -> list[str]:
        if not self.collection:
            return []
        results = self.collection.query(query_embeddings=[self.embed(query)], n_results=n)
        return results["documents"][0] if results["documents"] else []


class AgentWithMemory:
    def __init__(self, client: OpenAIClient):
        self.client = client
        self.short_term = ShortTermMemory(max_messages=12)
        self.long_term = LongTermMemory(client)

    def chat(self, user_input: str) -> str:
        relevant = self.long_term.retrieve(user_input)
        if relevant:
            context = "Relevant memory:\n" + "\n".join(f"- {r}" for r in relevant)
            self.short_term.add("system", context)

        self.short_term.add("user", user_input)
        messages = self.short_term.get()
        response = self.client.chat_completion(messages=messages, max_tokens=200)
        answer = response["choices"][0]["message"]["content"]
        self.short_term.add("assistant", answer)
        self.long_term.store(f"User: {user_input}\nAssistant: {answer}")
        return answer


def main():
    configure_logging()
    client = OpenAIClient()
    agent = AgentWithMemory(client)

    print("Agent:", agent.chat("My name is Alice and I work on backend systems."))
    print("Agent:", agent.chat("What do I work on?"))
    print("Agent:", agent.chat("Suggest a logging strategy for my team."))


if __name__ == "__main__":
    main()
