package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

type agentState struct {
	Question string
	Context  string
	Action   string
	Answer   string
}

type graphNode func(state *agentState) *agentState

type stateGraph struct {
	nodes     map[string]graphNode
	edges     map[string]string
	condition map[string]map[string]string
	start     string
	end       string
}

func newStateGraph() *stateGraph {
	return &stateGraph{
		nodes:     map[string]graphNode{},
		edges:     map[string]string{},
		condition: map[string]map[string]string{},
	}
}

func (g *stateGraph) addNode(name string, fn graphNode) {
	g.nodes[name] = fn
}

func (g *stateGraph) addEdge(from, to string) {
	g.edges[from] = to
}

func (g *stateGraph) addConditionalEdges(from string, branches map[string]string) {
	g.condition[from] = branches
}

func (g *stateGraph) setStart(name string) {
	g.start = name
}

func (g *stateGraph) setEnd(name string) {
	g.end = name
}

func (g *stateGraph) invoke(initial *agentState) *agentState {
	state := initial
	current := g.start
	for current != "" {
		fn := g.nodes[current]
		updates := fn(state)
		if updates.Question != "" {
			state.Question = updates.Question
		}
		if updates.Context != "" {
			state.Context = updates.Context
		}
		if updates.Action != "" {
			state.Action = updates.Action
		}
		if updates.Answer != "" {
			state.Answer = updates.Answer
		}

		if current == g.end {
			break
		}

		if next, ok := g.condition[current]; ok {
			branch := state.Action
			if branch == "" {
				branch = "answer_directly"
			}
			current = next[branch]
		} else {
			current = g.edges[current]
		}
	}
	return state
}

func makeNodes(client *openaiclient.Client) (graphNode, graphNode, graphNode, graphNode) {
	retrieveContext := func(state *agentState) *agentState {
		context := fmt.Sprintf(
			"Documents related to: %s\n- Agent engineering relies on composable patterns.\n- LangGraph adds structure to agent loops with states and edges.",
			state.Question,
		)
		log.Printf("Retrieved context for: %s", state.Question)
		return &agentState{Context: context}
	}

	decideAction := func(state *agentState) *agentState {
		if client == nil {
			log.Println("No LLM; using deterministic action fallback")
			return &agentState{Action: "answer_directly"}
		}
		prompt := fmt.Sprintf(
			"You are a routing agent. Given the user question and retrieved context, choose the next action: 'answer_directly' or 'search_more'. Reply with exactly one of those two strings, nothing else.\n\nQuestion: %s\nContext: %s",
			state.Question, state.Context,
		)
		resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(10),
		})
		action := "answer_directly"
		if err == nil && len(resp.Choices) > 0 {
			a := strings.ToLower(strings.TrimSpace(resp.Choices[0].Message.Content))
			if a == "answer_directly" || a == "search_more" {
				action = a
			}
		}
		log.Printf("Decided action: %s", action)
		return &agentState{Action: action}
	}

	webSearch := func(state *agentState) *agentState {
		log.Println("Performing mock web search")
		extra := "\n- Recent web result confirms LangGraph 0.2 adds checkpointing."
		return &agentState{Context: state.Context + extra}
	}

	generateAnswer := func(state *agentState) *agentState {
		if client == nil {
			log.Println("No LLM; using deterministic answer fallback")
			return &agentState{Answer: fmt.Sprintf("Answer for '%s': based on the retrieved context, LangGraph helps structure agent workflows with states and edges.", state.Question)}
		}
		prompt := fmt.Sprintf("Use the context below to answer the question concisely.\n\nQuestion: %s\n\nContext:\n%s\n\nAnswer:", state.Question, state.Context)
		resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
			Temperature: floatPtr(0.3),
			MaxTokens:   intPtr(200),
		})
		answer := ""
		if err == nil && len(resp.Choices) > 0 {
			answer = strings.TrimSpace(resp.Choices[0].Message.Content)
		}
		return &agentState{Answer: answer}
	}

	return retrieveContext, decideAction, webSearch, generateAnswer
}

func buildGraph(client *openaiclient.Client) *stateGraph {
	retrieveContext, decideAction, webSearch, generateAnswer := makeNodes(client)

	g := newStateGraph()
	g.addNode("retrieve_context", retrieveContext)
	g.addNode("decide_action", decideAction)
	g.addNode("web_search", webSearch)
	g.addNode("generate_answer", generateAnswer)

	g.setStart("retrieve_context")
	g.addEdge("retrieve_context", "decide_action")
	g.addConditionalEdges("decide_action", map[string]string{
		"answer_directly": "generate_answer",
		"search_more":     "web_search",
	})
	g.addEdge("web_search", "generate_answer")
	g.setEnd("generate_answer")

	return g
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func main() {
	cfg := openaiclient.LoadConfig()
	var client *openaiclient.Client
	if cfg.APIKey != "" || strings.HasPrefix(cfg.BaseURL, "http://localhost") {
		client = openaiclient.NewClient()
	} else {
		log.Println("LLM client disabled: OPENAI_API_KEY not set")
	}

	graph := buildGraph(client)
	question := "How does LangGraph help build reliable agents?"
	fmt.Println("Question:", question)
	result := graph.invoke(&agentState{Question: question})
	fmt.Println("\nAction chosen:", result.Action)
	fmt.Println("Context:\n", result.Context)
	fmt.Println("\nAnswer:", result.Answer)
}
