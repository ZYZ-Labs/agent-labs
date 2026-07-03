"""LangGraph practice: build a simple state graph.

Nodes: retrieve_context -> decide_action -> [web_search] -> generate_answer.
"""
import logging
import sys
from pathlib import Path
from typing import TypedDict

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "shared" / "config"))

from openai_client import OpenAIClient, configure_logging

logger = logging.getLogger("langgraph-practice")

try:
    from langgraph.graph import END, START, StateGraph
except ImportError as exc:
    logger.error(
        "LangGraph is not installed. Run: pip install -r requirements.txt"
    )
    raise


class AgentState(TypedDict):
    question: str
    context: str
    action: str
    answer: str


def make_nodes(client: OpenAIClient | None):
    def retrieve_context(state: AgentState) -> dict[str, str]:
        question = state["question"]
        # Mock retrieval; in production this would query a vector DB or search index.
        context = (
            f"Documents related to: {question}\n"
            "- Agent engineering relies on composable patterns.\n"
            "- LangGraph adds structure to agent loops with states and edges."
        )
        logger.info("Retrieved context for: %s", question)
        return {"context": context}

    def decide_action(state: AgentState) -> dict[str, str]:
        if not client:
            logger.info("No LLM; using deterministic action fallback")
            return {"action": "answer_directly"}
        prompt = (
            "You are a routing agent. Given the user question and retrieved context, "
            "choose the next action: 'answer_directly' or 'search_more'. "
            "Reply with exactly one of those two strings, nothing else.\n\n"
            f"Question: {state['question']}\nContext: {state['context']}"
        )
        resp = client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            temperature=0.0,
            max_tokens=10,
        )
        action = resp["choices"][0]["message"]["content"].strip().lower()
        if action not in ("answer_directly", "search_more"):
            action = "answer_directly"
        logger.info("Decided action: %s", action)
        return {"action": action}

    def web_search(state: AgentState) -> dict[str, str]:
        # Mock web search.
        logger.info("Performing mock web search")
        extra = "\n- Recent web result confirms LangGraph 0.2 adds checkpointing."
        return {"context": state["context"] + extra}

    def generate_answer(state: AgentState) -> dict[str, str]:
        if not client:
            logger.info("No LLM; using deterministic answer fallback")
            return {
                "answer": (
                    f"Answer for '{state['question']}': based on the retrieved context, "
                    "LangGraph helps structure agent workflows with states and edges."
                )
            }
        prompt = (
            "Use the context below to answer the question concisely.\n\n"
            f"Question: {state['question']}\n\n"
            f"Context:\n{state['context']}\n\nAnswer:"
        )
        resp = client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            temperature=0.3,
            max_tokens=200,
        )
        return {"answer": resp["choices"][0]["message"]["content"].strip()}

    return retrieve_context, decide_action, web_search, generate_answer


def build_graph(client: OpenAIClient | None):
    (
        retrieve_context,
        decide_action,
        web_search,
        generate_answer,
    ) = make_nodes(client)

    builder = StateGraph(AgentState)
    builder.add_node("retrieve_context", retrieve_context)
    builder.add_node("decide_action", decide_action)
    builder.add_node("web_search", web_search)
    builder.add_node("generate_answer", generate_answer)

    builder.add_edge(START, "retrieve_context")
    builder.add_edge("retrieve_context", "decide_action")
    builder.add_conditional_edges(
        "decide_action",
        lambda state: state["action"],
        {
            "answer_directly": "generate_answer",
            "search_more": "web_search",
        },
    )
    builder.add_edge("web_search", "generate_answer")
    builder.add_edge("generate_answer", END)

    return builder.compile()


def main() -> None:
    configure_logging()
    client = None
    try:
        client = OpenAIClient()
    except ValueError as exc:
        logger.warning("LLM client disabled: %s", exc)

    graph = build_graph(client)
    question = "How does LangGraph help build reliable agents?"
    print("Question:", question)
    result = graph.invoke({"question": question})
    print("\nAction chosen:", result.get("action"))
    print("Context:\n", result.get("context"))
    print("\nAnswer:", result.get("answer"))


if __name__ == "__main__":
    main()
