package main

import (
	"encoding/json"
	"fmt"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

var modelPricing = map[string]map[string]float64{
	"gpt-4o":       {"input": 5.00, "output": 15.00},
	"gpt-4o-mini":  {"input": 0.15, "output": 0.60},
	"gpt-3.5-turbo": {"input": 0.50, "output": 1.50},
}

func estimateCost(model string, promptTokens, completionTokens int) float64 {
	pricing := modelPricing[model]
	if pricing == nil {
		pricing = map[string]float64{"input": 0.0, "output": 0.0}
	}
	return (float64(promptTokens)*pricing["input"] + float64(completionTokens)*pricing["output"]) / 1_000_000
}

func compareCosts(promptTokens, completionTokens int) []map[string]interface{} {
	var rows []map[string]interface{}
	for model := range modelPricing {
		rows = append(rows, map[string]interface{}{
			"model":               model,
			"estimated_cost_usd":  round(estimateCost(model, promptTokens, completionTokens), 6),
		})
	}
	return rows
}

func round(x float64, places int) float64 {
	factor := 1.0
	for i := 0; i < places; i++ {
		factor *= 10
	}
	return float64(int(x*factor+0.5)) / factor
}

func chooseModel(taskDescription string) string {
	desc := strings.ToLower(taskDescription)
	complexSignals := []string{"architecture", "design doc", "refactor", "complex", "multistep", "review"}
	for _, signal := range complexSignals {
		if strings.Contains(desc, signal) {
			return "gpt-4o"
		}
	}
	return "gpt-4o-mini"
}

func countTokens(text, model string) int {
	// Go has no tiktoken; use rough word count fallback.
	return len(strings.Fields(text))
}

func runCachedPromptAgent(client *openaiclient.Client, userMessage string) map[string]interface{} {
	systemPrefix := "You are a concise coding assistant. Always answer in one sentence. Prefer short variable names and simple algorithms."

	fullPrompt := systemPrefix + "\n" + userMessage
	fullTokens := countTokens(fullPrompt, client.Config.Model)
	userTokens := countTokens(userMessage, client.Config.Model)

	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages: []openaiclient.Message{
			{Role: "system", Content: systemPrefix},
			{Role: "user", Content: userMessage},
		},
		MaxTokens:   intPtr(200),
		Temperature: floatPtr(0.0),
	})
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	content := resp.Choices[0].Message.Content
	completionTokens := resp.Usage.CompletionTokens
	if completionTokens == 0 {
		completionTokens = countTokens(content, client.Config.Model)
	}

	estimatedCostCached := estimateCost(client.Config.Model, userTokens, completionTokens)
	estimatedCostUncached := estimateCost(client.Config.Model, fullTokens, completionTokens)

	return map[string]interface{}{
		"model":                        client.Config.Model,
		"user_message":                 userMessage,
		"full_prompt_tokens":           fullTokens,
		"new_input_tokens":             userTokens,
		"completion_tokens":            completionTokens,
		"estimated_cost_cached_usd":    round(estimatedCostCached, 6),
		"estimated_cost_uncached_usd":  round(estimatedCostUncached, 6),
		"response":                     content,
	}
}

func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

func main() {
	client := openaiclient.NewClient()

	fmt.Println("Cost comparison for 1000 input + 500 output tokens:")
	for _, row := range compareCosts(1000, 500) {
		fmt.Printf("  %s: $%.6f\n", row["model"], row["estimated_cost_usd"])
	}

	tasks := []string{
		"Summarize this paragraph in one sentence.",
		"Generate an architecture design doc for a payment gateway.",
		"Refactor this Python function to use async IO.",
	}
	fmt.Println("\nRouting decisions:")
	for _, task := range tasks {
		model := chooseModel(task)
		fmt.Printf("  [%s] %s\n", model, task)
	}

	userMessage := "What is the capital of France?"
	fmt.Printf("\nCached prompt example:\nUser: %s\n", userMessage)
	result := runCachedPromptAgent(client, userMessage)
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}
