import { OpenAIClient } from "../../../shared/config/openai_client";

interface AgentState {
  question: string;
  context: string;
  action: string;
  answer: string;
}

type NodeFn = (state: AgentState) => Promise<Partial<AgentState>> | Partial<AgentState>;

const START = "__start__";
const END = "__end__";

class SimpleStateGraph {
  private nodes: Record<string, NodeFn> = {};
  private edges: Record<string, string> = {};
  private conditionalEdges: Record<
    string,
    { fn: (state: AgentState) => string; mapping: Record<string, string> }
  > = {};

  addNode(name: string, fn: NodeFn): this {
    this.nodes[name] = fn;
    return this;
  }

  addEdge(from: string, to: string): this {
    this.edges[from] = to;
    return this;
  }

  addConditionalEdges(
    from: string,
    fn: (state: AgentState) => string,
    mapping: Record<string, string>
  ): this {
    this.conditionalEdges[from] = { fn, mapping };
    return this;
  }

  compile(): { invoke: (state: Partial<AgentState>) => Promise<AgentState> } {
    return {
      invoke: async (initialState: Partial<AgentState>) => {
        const state: AgentState = {
          question: initialState.question || "",
          context: initialState.context || "",
          action: initialState.action || "",
          answer: initialState.answer || "",
        };
        let current = START;
        while (current !== END) {
          let next: string;
          if (current === START) {
            next = this.edges[START];
          } else {
            const nodeFn = this.nodes[current];
            if (!nodeFn) throw new Error(`Unknown node: ${current}`);
            const update = await Promise.resolve(nodeFn(state));
            Object.assign(state, update);
            if (this.conditionalEdges[current]) {
              const key = this.conditionalEdges[current].fn(state);
              next = this.conditionalEdges[current].mapping[key] || END;
            } else if (this.edges[current]) {
              next = this.edges[current];
            } else {
              next = END;
            }
          }
          current = next;
        }
        return state;
      },
    };
  }
}

function makeNodes(client: OpenAIClient | null) {
  async function retrieveContext(state: AgentState): Promise<Partial<AgentState>> {
    const question = state.question;
    const context =
      `Documents related to: ${question}\n` +
      "- Agent engineering relies on composable patterns.\n" +
      "- LangGraph adds structure to agent loops with states and edges.";
    console.log(`Retrieved context for: ${question}`);
    return { context };
  }

  async function decideAction(state: AgentState): Promise<Partial<AgentState>> {
    if (!client) {
      console.log("No LLM; using deterministic action fallback");
      return { action: "answer_directly" };
    }
    const prompt =
      "You are a routing agent. Given the user question and retrieved context, " +
      "choose the next action: 'answer_directly' or 'search_more'. " +
      "Reply with exactly one of those two strings, nothing else.\n\n" +
      `Question: ${state.question}\nContext: ${state.context}`;
    const resp = await client.chatCompletion({
      messages: [{ role: "user", content: prompt }],
      temperature: 0.0,
      max_tokens: 10,
    });
    let action = resp.choices[0].message.content.trim().toLowerCase();
    if (action !== "answer_directly" && action !== "search_more") {
      action = "answer_directly";
    }
    console.log(`Decided action: ${action}`);
    return { action };
  }

  async function webSearch(state: AgentState): Promise<Partial<AgentState>> {
    console.log("Performing mock web search");
    const extra = "\n- Recent web result confirms LangGraph 0.2 adds checkpointing.";
    return { context: state.context + extra };
  }

  async function generateAnswer(state: AgentState): Promise<Partial<AgentState>> {
    if (!client) {
      console.log("No LLM; using deterministic answer fallback");
      return {
        answer:
          `Answer for '${state.question}': based on the retrieved context, ` +
          "LangGraph helps structure agent workflows with states and edges.",
      };
    }
    const prompt =
      "Use the context below to answer the question concisely.\n\n" +
      `Question: ${state.question}\n\n` +
      `Context:\n${state.context}\n\nAnswer:`;
    const resp = await client.chatCompletion({
      messages: [{ role: "user", content: prompt }],
      temperature: 0.3,
      max_tokens: 200,
    });
    return { answer: resp.choices[0].message.content.trim() };
  }

  return { retrieveContext, decideAction, webSearch, generateAnswer };
}

function buildGraph(client: OpenAIClient | null) {
  const { retrieveContext, decideAction, webSearch, generateAnswer } = makeNodes(client);

  const builder = new SimpleStateGraph();
  builder.addNode("retrieve_context", retrieveContext);
  builder.addNode("decide_action", decideAction);
  builder.addNode("web_search", webSearch);
  builder.addNode("generate_answer", generateAnswer);

  builder.addEdge(START, "retrieve_context");
  builder.addEdge("retrieve_context", "decide_action");
  builder.addConditionalEdges(
    "decide_action",
    (state) => state.action,
    {
      answer_directly: "generate_answer",
      search_more: "web_search",
    }
  );
  builder.addEdge("web_search", "generate_answer");
  builder.addEdge("generate_answer", END);

  return builder.compile();
}

async function main() {
  let client: OpenAIClient | null = null;
  try {
    client = new OpenAIClient();
  } catch (exc: any) {
    console.warn("LLM client disabled:", exc.message);
  }

  const graph = buildGraph(client);
  const question = "How does LangGraph help build reliable agents?";
  console.log("Question:", question);
  const result = await graph.invoke({ question });
  console.log("\nAction chosen:", result.action);
  console.log("Context:\n", result.context);
  console.log("\nAnswer:", result.answer);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
