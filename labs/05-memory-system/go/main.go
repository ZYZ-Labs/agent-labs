package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

type shortTermMemory struct {
	messages    []openaiclient.Message
	maxMessages int
}

func newShortTermMemory(maxMessages int) *shortTermMemory {
	return &shortTermMemory{maxMessages: maxMessages}
}

func (m *shortTermMemory) add(role, content string) {
	m.messages = append(m.messages, openaiclient.Message{Role: role, Content: content})
	if len(m.messages) > m.maxMessages {
		keep := 0
		if len(m.messages) > 0 && m.messages[0].Role == "system" {
			keep = 1
		}
		m.messages = append(m.messages[:keep], m.messages[keep+1:]...)
	}
}

func (m *shortTermMemory) get() []openaiclient.Message {
	out := make([]openaiclient.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

type longTermMemory struct {
	client *openaiclient.Client
}

func newLongTermMemory(client *openaiclient.Client) *longTermMemory {
	return &longTermMemory{client: client}
}

func (ltm *longTermMemory) embed(text string) []float64 {
	if ltm.client != nil {
		resp, err := ltm.client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages: []openaiclient.Message{
				{Role: "user", Content: "Summarize in one sentence for retrieval: " + text},
			},
			MaxTokens: intPtr(20),
		})
		if err == nil && len(resp.Choices) > 0 {
			text = resp.Choices[0].Message.Content
		}
	}
	vec := make([]float64, 64)
	h := sha256.Sum256([]byte(text))
	for i, b := range h {
		vec[i%64] += float64(b) / 255.0
	}
	return vec
}

func (ltm *longTermMemory) store(text string, metadata map[string]string) {
	fmt.Println("[Long-term memory] Chroma not available, skipping store.")
}

func (ltm *longTermMemory) retrieve(query string, n int) []string {
	return nil
}

type agentWithMemory struct {
	client    *openaiclient.Client
	shortTerm *shortTermMemory
	longTerm  *longTermMemory
}

func newAgentWithMemory(client *openaiclient.Client) *agentWithMemory {
	return &agentWithMemory{
		client:    client,
		shortTerm: newShortTermMemory(12),
		longTerm:  newLongTermMemory(client),
	}
}

func (a *agentWithMemory) chat(userInput string) string {
	relevant := a.longTerm.retrieve(userInput)
	if len(relevant) > 0 {
		context := "Relevant memory:\n"
		for _, r := range relevant {
			context += "- " + r + "\n"
		}
		a.shortTerm.add("system", strings.TrimSpace(context))
	}

	a.shortTerm.add("user", userInput)
	resp, err := a.client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:  a.shortTerm.get(),
		MaxTokens: intPtr(200),
	})
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	answer := resp.Choices[0].Message.Content
	a.shortTerm.add("assistant", answer)
	a.longTerm.store(fmt.Sprintf("User: %s\nAssistant: %s", userInput, answer), nil)
	return answer
}

func intPtr(i int) *int {
	return &i
}

func main() {
	client := openaiclient.NewClient()
	agent := newAgentWithMemory(client)

	fmt.Println("Agent:", agent.chat("My name is Alice and I work on backend systems."))
	fmt.Println("Agent:", agent.chat("What do I work on?"))
	fmt.Println("Agent:", agent.chat("Suggest a logging strategy for my team."))
}
