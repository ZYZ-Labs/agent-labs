import express, { Request, Response } from "express";
import { OpenAIClient } from "../../../shared/config/openai_client";

const app = express();
app.use(express.json());

const apiKey = process.env.OPENAI_API_KEY || "";
if (!apiKey) {
  console.warn("Warning: OPENAI_API_KEY is not set. /chat will return 503 until configured.");
}

const client = new OpenAIClient();

app.get("/health", (_req: Request, res: Response) => {
  res.json({ status: "ok", configured: Boolean(apiKey) });
});

app.post("/chat", async (req: Request, res: Response) => {
  if (!apiKey) {
    res.status(503).json({ error: "OPENAI_API_KEY is not configured" });
    return;
  }
  const { message, model } = req.body;
  if (!message || !message.trim()) {
    res.status(400).json({ error: "message is required" });
    return;
  }
  try {
    const response = await client.chatCompletion({
      messages: [{ role: "user", content: message }],
      model,
      max_tokens: 300,
      temperature: 0,
    });
    const reply = client.extractMessage(response).content || "";
    res.json({ reply, model: model || client.config.model });
  } catch (err) {
    res.status(502).json({ error: `Upstream error: ${err}` });
  }
});

const port = process.env.PORT || 8000;
app.listen(port, () => {
  console.log(`Server listening on :${port}`);
});
