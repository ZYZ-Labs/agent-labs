package main

import (
	"encoding/json"
	"fmt"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

var schema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"service_name": map[string]interface{}{"type": "string"},
		"port":         map[string]interface{}{"type": "integer", "minimum": 1024, "maximum": 65535},
		"replicas":     map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 10},
		"env":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
	},
	"required": []string{"service_name", "port", "replicas"},
}

func validateConfig(config map[string]interface{}) string {
	if config == nil {
		return "Config must be a JSON object."
	}
	for _, key := range []string{"service_name", "port", "replicas"} {
		if _, ok := config[key]; !ok {
			return fmt.Sprintf("Missing required field: %s", key)
		}
	}
	port, ok := config["port"].(float64)
	if !ok {
		return "port must be an integer."
	}
	if port < 1024 || port > 65535 {
		return "port must be between 1024 and 65535."
	}
	replicas, ok := config["replicas"].(float64)
	if !ok {
		return "replicas must be an integer."
	}
	if replicas < 1 || replicas > 10 {
		return "replicas must be between 1 and 10."
	}
	return ""
}

func generateWithRecovery(client *openaiclient.Client, request string, maxRetries int) map[string]interface{} {
	schemaJSON, _ := json.Marshal(schema)
	messages := []openaiclient.Message{
		{
			Role: "system",
			Content: fmt.Sprintf(
				"You are a configuration generator. Return only valid JSON matching this schema: %s. No markdown, no explanation.",
				string(schemaJSON),
			),
		},
		{Role: "user", Content: request},
	}

	temp := 0.2
	maxTokens := 300

	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("\n--- Attempt %d ---\n", attempt)
		resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages:    messages,
			Temperature: &temp,
			MaxTokens:   &maxTokens,
		})
		if err != nil {
			fmt.Printf("Chat error: %v\n", err)
			continue
		}
		raw := resp.Choices[0].Message.Content
		fmt.Println("Raw output:", raw)

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			errorMsg := fmt.Sprintf("Invalid JSON: %v", err)
			fmt.Println(errorMsg)
			messages = append(messages, openaiclient.Message{Role: "assistant", Content: raw})
			messages = append(messages, openaiclient.Message{Role: "user", Content: fmt.Sprintf("That was not valid JSON. %s Please retry.", errorMsg)})
			continue
		}

		if errStr := validateConfig(parsed); errStr == "" {
			return parsed
		} else {
			fmt.Println("Validation error:", errStr)
			messages = append(messages, openaiclient.Message{Role: "assistant", Content: raw})
			messages = append(messages, openaiclient.Message{Role: "user", Content: fmt.Sprintf("Validation failed: %s. Fix the JSON and retry.", errStr)})
		}
	}

	panic("Failed to generate valid config after max retries.")
}

func main() {
	client := openaiclient.NewClient()
	request := "Create a config for a payment-api service on port 8080 with 3 replicas and env vars LOG_LEVEL=info,DB_URL=postgres."
	config := generateWithRecovery(client, request, 3)
	fmt.Println("\nFinal valid config:")
	b, _ := json.MarshalIndent(config, "", "  ")
	fmt.Println(string(b))
}
