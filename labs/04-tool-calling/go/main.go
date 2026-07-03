package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

var tools = []map[string]interface{}{
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_weather",
			"description": "Get current weather for a city.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city": map[string]interface{}{"type": "string", "description": "City name"},
				},
				"required": []string{"city"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "search_notes",
			"description": "Search project notes by keyword.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search keyword"},
				},
				"required": []string{"query"},
			},
		},
	},
}

var notes = []map[string]string{
	{"title": "MCP design", "content": "MCP uses Resources, Tools, Prompts, and Sampling primitives."},
	{"title": "LSP basics", "content": "LSP speaks JSON-RPC over stdio or sockets."},
	{"title": "Agent memory", "content": "Short-term memory lives in the context window; long-term in vectors."},
}

func getWeather(city string) string {
	b, _ := json.Marshal(map[string]interface{}{"city": city, "temperature_c": 22, "condition": "sunny"})
	return string(b)
}

func searchNotes(query string) string {
	var results []map[string]string
	q := strings.ToLower(query)
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n["title"]), q) || strings.Contains(strings.ToLower(n["content"]), q) {
			results = append(results, n)
		}
	}
	b, _ := json.Marshal(results)
	return string(b)
}

var toolFunctions = map[string]func(map[string]interface{}) string{
	"get_weather": func(args map[string]interface{}) string {
		city, _ := args["city"].(string)
		return getWeather(city)
	},
	"search_notes": func(args map[string]interface{}) string {
		query, _ := args["query"].(string)
		return searchNotes(query)
	},
}

// rawResponse captures tool_calls which the shared wrapper does not expose.
type rawResponse struct {
	Choices []struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func chatCompletionWithTools(client *openaiclient.Client, messages []map[string]interface{}) (*rawResponse, error) {
	reqBody := map[string]interface{}{
		"model":      client.Config.Model,
		"messages":   messages,
		"tools":      tools,
		"tool_choice": "auto",
		"temperature": 0.0,
		"max_tokens":  300,
		"stream":      false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", client.Config.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.Config.APIKey)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var result rawResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func runToolAgent(client *openaiclient.Client, userMessage string, maxIterations int) string {
	messages := []map[string]interface{}{
		{"role": "user", "content": userMessage},
	}

	for i := 0; i < maxIterations; i++ {
		resp, err := chatCompletionWithTools(client, messages)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		message := resp.Choices[0].Message
		messages = append(messages, map[string]interface{}{
			"role":    message.Role,
			"content": message.Content,
			"tool_calls": func() []map[string]interface{} {
				var calls []map[string]interface{}
				for _, tc := range message.ToolCalls {
					calls = append(calls, map[string]interface{}{
						"id": tc.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      tc.Function.Name,
							"arguments": tc.Function.Arguments,
						},
					})
				}
				return calls
			}(),
		})

		if len(message.ToolCalls) == 0 {
			return message.Content
		}

		for _, tc := range message.ToolCalls {
			name := tc.Function.Name
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			fmt.Printf("[Tool call] %s(%v)\n", name, args)
			fn := toolFunctions[name]
			var result string
			if fn != nil {
				result = fn(args)
			} else {
				b, _ := json.Marshal(map[string]string{"error": "unknown tool " + name})
				result = string(b)
			}
			messages = append(messages, map[string]interface{}{
				"role":       "tool",
				"tool_call_id": tc.ID,
				"name":       name,
				"content":    result,
			})
			fmt.Printf("[Tool result] %s\n", result)
		}
	}

	return "Reached max iterations."
}

func main() {
	client := openaiclient.NewClient()
	question := "What's the weather in Shanghai? Also, find me notes about MCP."
	fmt.Println("User:", question)
	answer := runToolAgent(client, question, 5)
	fmt.Println("\nAssistant:", answer)
}
