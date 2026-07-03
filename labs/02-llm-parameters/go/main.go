package main

import (
	"fmt"
	"log"

	"github.com/ZYZ-Labs/agent-labs/shared/config"
)

func main() {
	client := openaiclient.NewClient()

	messages := []openaiclient.Message{
		{Role: "system", Content: "You are a helpful coding assistant. Be concise."},
		{
			Role: "user",
			Content: "List three benefits of using state machines to model agent workflows. Answer in at most two sentences.",
		},
	}

	run := func(name string, req openaiclient.ChatCompletionRequest) {
		fmt.Printf("\n=== %s ===\n", name)
		req.Messages = messages
		resp, err := client.ChatCompletion(req)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			return
		}
		for i, choice := range resp.Choices {
			if len(resp.Choices) > 1 {
				fmt.Printf("  Choice %d: %s\n", i, choice.Message.Content)
			} else {
				fmt.Printf("  Output: %s\n", choice.Message.Content)
			}
		}
		fmt.Printf("  Finish reason: %s\n", resp.Choices[0].FinishReason)
		fmt.Printf("  Usage: %+v\n", resp.Usage)
	}

	run("Default", openaiclient.ChatCompletionRequest{MaxTokens: intPtr(120)})
	run("High temperature", openaiclient.ChatCompletionRequest{Temperature: floatPtr(1.2), MaxTokens: intPtr(120)})
	run("Low temperature", openaiclient.ChatCompletionRequest{Temperature: floatPtr(0.0), MaxTokens: intPtr(120)})
	run("top_p", openaiclient.ChatCompletionRequest{TopP: floatPtr(0.3), MaxTokens: intPtr(120)})
	run("Frequency penalty", openaiclient.ChatCompletionRequest{FrequencyPenalty: floatPtr(1.0), MaxTokens: intPtr(120)})
	run("Presence penalty", openaiclient.ChatCompletionRequest{PresencePenalty: floatPtr(1.0), MaxTokens: intPtr(120)})
	run("Stop sequence", openaiclient.ChatCompletionRequest{Stop: []string{"."}, MaxTokens: intPtr(120)})
	run("Seed", openaiclient.ChatCompletionRequest{Seed: intPtr(42), Temperature: floatPtr(0.0), MaxTokens: intPtr(120)})
}

func intPtr(v int) *int   { return &v }
func floatPtr(v float64) *float64 { return &v }
