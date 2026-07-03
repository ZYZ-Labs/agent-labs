package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

var agents = map[string]map[string]string{
	"coding": {
		"system":   "You are a coding assistant. Answer the user's programming question with concise code and explanation.",
		"fallback": "Use Python functions and add type hints for clarity.",
	},
	"writing": {
		"system":   "You are a writing assistant. Improve clarity, grammar, and tone.",
		"fallback": "Use short sentences and active voice.",
	},
	"math": {
		"system":   "You are a math assistant. Solve the problem step by step.",
		"fallback": "Break the problem into smaller equations.",
	},
}

type multiAgentSystem struct {
	client *openaiclient.Client
}

func newMultiAgentSystem(client *openaiclient.Client) *multiAgentSystem {
	return &multiAgentSystem{client: client}
}

func (s *multiAgentSystem) route(request string) []string {
	if s.client == nil {
		log.Println("No LLM; routing by keyword fallback")
		lowered := strings.ToLower(request)
		var topics []string
		codingKeys := []string{"code", "python", "function", "error"}
		writingKeys := []string{"write", "essay", "grammar", "draft"}
		mathKeys := []string{"math", "calculate", "equation", "sum"}
		for _, k := range codingKeys {
			if strings.Contains(lowered, k) {
				topics = append(topics, "coding")
				break
			}
		}
		for _, k := range writingKeys {
			if strings.Contains(lowered, k) {
				topics = append(topics, "writing")
				break
			}
		}
		for _, k := range mathKeys {
			if strings.Contains(lowered, k) {
				topics = append(topics, "math")
				break
			}
		}
		if len(topics) == 0 {
			topics = []string{"writing"}
		}
		return topics
	}

	resp, err := s.client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages: []openaiclient.Message{
			{Role: "system", Content: "You are a router. Given a user request, choose one or more specialist topics from: coding, writing, math. Reply with a JSON array of strings only, e.g. [\"coding\"]."},
			{Role: "user", Content: request},
		},
		Temperature:    floatPtr(0.0),
		MaxTokens:      intPtr(50),
		ResponseFormat: map[string]string{"type": "json_object"},
	})
	if err != nil {
		return []string{"writing"}
	}
	content := resp.Choices[0].Message.Content
	var parsed interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return []string{"writing"}
	}
	switch v := parsed.(type) {
	case []interface{}:
		var out []string
		for _, t := range v {
			if s, ok := t.(string); ok && agents[s] != nil {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	case map[string]interface{}:
		if topics, ok := v["topics"].([]interface{}); ok {
			var out []string
			for _, t := range topics {
				if s, ok := t.(string); ok && agents[s] != nil {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return []string{"writing"}
}

func (s *multiAgentSystem) worker(topic, request string) map[string]interface{} {
	cfg := agents[topic]
	var answer string
	if s.client == nil {
		answer = cfg["fallback"]
	} else {
		resp, err := s.client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages: []openaiclient.Message{
				{Role: "system", Content: cfg["system"]},
				{Role: "user", Content: request},
			},
			Temperature: floatPtr(0.3),
			MaxTokens:   intPtr(200),
		})
		if err != nil {
			answer = fmt.Sprintf("Error: %v", err)
		} else {
			answer = strings.TrimSpace(resp.Choices[0].Message.Content)
		}
	}
	return map[string]interface{}{"topic": topic, "answer": answer}
}

func (s *multiAgentSystem) aggregate(request string, responses []map[string]interface{}) string {
	if s.client == nil {
		var parts []string
		for _, r := range responses {
			parts = append(parts, fmt.Sprintf("### %s\n%s", r["topic"], r["answer"]))
		}
		return strings.Join(parts, "\n\n")
	}

	var parts []string
	for _, r := range responses {
		parts = append(parts, fmt.Sprintf("### %s\n%s", r["topic"], r["answer"]))
	}
	combined := strings.Join(parts, "\n\n")
	resp, err := s.client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages: []openaiclient.Message{
			{Role: "system", Content: "You are an aggregator. Combine the specialist answers into a single coherent response."},
			{Role: "user", Content: fmt.Sprintf("User request: %s\n\nSpecialist answers:\n%s\n\nProvide a final answer.", request, combined)},
		},
		Temperature: floatPtr(0.3),
		MaxTokens:   intPtr(300),
	})
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

func (s *multiAgentSystem) run(request string) map[string]interface{} {
	topics := s.route(request)
	log.Printf("Routed to: %v", topics)
	var responses []map[string]interface{}
	for _, t := range topics {
		responses = append(responses, s.worker(t, request))
	}
	final := s.aggregate(request, responses)
	return map[string]interface{}{"topics": topics, "responses": responses, "final_answer": final}
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

	system := newMultiAgentSystem(client)
	request := "How do I write a Python function that retries a failing operation?"
	fmt.Println("User request:", request)
	result := system.run(request)
	fmt.Println("\nRouted to:", result["topics"])
	fmt.Println("\nSpecialist answers:")
	for _, r := range result["responses"].([]map[string]interface{}) {
		answer := r["answer"].(string)
		if len(answer) > 200 {
			answer = answer[:200] + "..."
		}
		fmt.Printf("  [%s] %s\n", r["topic"], answer)
	}
	fmt.Println("\nFinal aggregated answer:")
	fmt.Println(result["final_answer"])
}
