import { createHash } from "crypto";
import { OpenAIClient, ChatMessage } from "../../../shared/config/openai_client";

let chromaModule: any;
try {
  chromaModule = require("chromadb");
} catch {
  chromaModule = null;
}

class ShortTermMemory {
  messages: ChatMessage[] = [];

  constructor(private maxMessages = 10) {}

  add(role: string, content: string): void {
    this.messages.push({ role: role as ChatMessage["role"], content });
    if (this.messages.length > this.maxMessages) {
      const keep = this.messages[0]?.role === "system" ? 1 : 0;
      this.messages = this.messages.slice(0, keep).concat(this.messages.slice(keep + 1));
    }
  }

  get(): ChatMessage[] {
    return this.messages.slice();
  }
}

class LongTermMemory {
  collection: any = null;

  constructor(private client: OpenAIClient, collectionName = "agent_memory") {
    if (chromaModule?.ChromaClient) {
      const db = new chromaModule.ChromaClient({ path: "http://localhost:8000" });
      db.getOrCreateCollection({ name: collectionName }).then((col: any) => {
        this.collection = col;
      }).catch((err: any) => {
        console.warn("[Long-term memory] Could not connect to Chroma:", err.message);
      });
    }
  }

  async embed(text: string): Promise<number[]> {
    const resp = await this.client.chatCompletion({
      messages: [
        {
          role: "user",
          content: `Summarize in one sentence for retrieval: ${text}`,
        },
      ],
      max_tokens: 20,
    });
    const summary = resp.choices[0].message.content;
    const digest = createHash("sha256").update(summary).digest();
    const vec = new Array(64).fill(0);
    for (let i = 0; i < digest.length; i++) {
      vec[i % 64] += digest[i] / 255.0;
    }
    return vec;
  }

  async store(text: string, metadata?: Record<string, any>): Promise<void> {
    if (!this.collection) {
      console.log("[Long-term memory] Chroma not available, skipping store.");
      return;
    }
    const docId = createHash("sha256").update(text).digest("hex").slice(0, 16);
    await this.collection.add({
      ids: [docId],
      documents: [text],
      metadatas: [metadata || {}],
      embeddings: [await this.embed(text)],
    });
  }

  async retrieve(query: string, n = 3): Promise<string[]> {
    if (!this.collection) return [];
    const results = await this.collection.query({
      queryEmbeddings: [await this.embed(query)],
      nResults: n,
    });
    return results.documents?.[0] || [];
  }
}

class AgentWithMemory {
  shortTerm: ShortTermMemory;
  longTerm: LongTermMemory;

  constructor(private client: OpenAIClient) {
    this.shortTerm = new ShortTermMemory(12);
    this.longTerm = new LongTermMemory(client);
  }

  async chat(userInput: string): Promise<string> {
    const relevant = await this.longTerm.retrieve(userInput);
    if (relevant.length > 0) {
      const context = "Relevant memory:\n" + relevant.map((r) => `- ${r}`).join("\n");
      this.shortTerm.add("system", context);
    }

    this.shortTerm.add("user", userInput);
    const messages = this.shortTerm.get();
    const response = await this.client.chatCompletion({ messages, max_tokens: 200 });
    const answer = response.choices[0].message.content;
    this.shortTerm.add("assistant", answer);
    await this.longTerm.store(`User: ${userInput}\nAssistant: ${answer}`);
    return answer;
  }
}

async function main() {
  const client = new OpenAIClient();
  const agent = new AgentWithMemory(client);

  console.log("Agent:", await agent.chat("My name is Alice and I work on backend systems."));
  console.log("Agent:", await agent.chat("What do I work on?"));
  console.log("Agent:", await agent.chat("Suggest a logging strategy for my team."));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
