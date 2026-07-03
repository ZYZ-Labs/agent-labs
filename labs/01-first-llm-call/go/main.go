package main

import (
	"fmt"
	"log"

	"github.com/ZYZ-Labs/agent-labs/shared/config"
)

func main() {
	client := openaiclient.NewClient()
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages: []openaiclient.Message{
			{Role: "system", Content: "You are a concise assistant."},
			{Role: "user", Content: "Explain what an AI agent is in one sentence."},
		},
		MaxTokens: intPtr(80),
	})
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	msg := client.ExtractMessage(resp)
	fmt.Println("Assistant:", msg.Content)
	fmt.Println("Finish reason:", resp.Choices[0].FinishReason)
	fmt.Printf("Usage: %+v\n", resp.Usage)
}

func intPtr(v int) *int { return &v }
