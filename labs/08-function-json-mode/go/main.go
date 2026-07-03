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

var eventSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"name":         map[string]interface{}{"type": "string"},
		"date":         map[string]interface{}{"type": "string"},
		"location":     map[string]interface{}{"type": "string"},
		"participants": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
	},
	"required": []string{"name", "date"},
}

var userPrompt = `
Extract the event details from this message as JSON:

"Join us for the AI Engineering Meetup on 2025-09-15 at the Shenzhen Hub.
Attendees: Alice, Bob, and Carol."
`

func validateEvent(data map[string]interface{}) (map[string]interface{}, error) {
	if data == nil {
		return nil, fmt.Errorf("parsed JSON is not an object")
	}
	var missing []string
	for _, f := range []string{"name", "date"} {
		if _, ok := data[f]; !ok {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required fields: %v", missing)
	}
	for key, value := range data {
		spec, ok := eventSchema["properties"].(map[string]interface{})[key]
		if !ok {
			continue
		}
		if s, ok := spec.(map[string]interface{}); ok && s["type"] == "array" {
			if _, ok := value.([]interface{}); !ok {
				return nil, fmt.Errorf("field '%s' should be an array", key)
			}
		}
	}
	return data, nil
}

func chatCompletionRaw(client *openaiclient.Client, reqBody map[string]interface{}) (map[string]interface{}, error) {
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
	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func extractWithJSONMode(client *openaiclient.Client) (map[string]interface{}, error) {
	resp, err := chatCompletionRaw(client, map[string]interface{}{
		"model": client.Config.Model,
		"messages": []map[string]interface{}{
			{
				"role": "system",
				"content": "You are a helpful parser. Return ONLY a JSON object with keys: name, date, location, participants.",
			},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.0,
		"max_tokens":      300,
		"stream":          false,
	})
	if err != nil {
		return nil, err
	}
	raw := extractContent(resp)
	fmt.Println("\n[JSON mode] raw output:")
	fmt.Println(raw)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	return validateEvent(data)
}

func extractWithFunctionCall(client *openaiclient.Client) (map[string]interface{}, error) {
	tool := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "extract_event",
			"description": "Extract event details from user text.",
			"parameters":  eventSchema,
		},
	}
	resp, err := chatCompletionRaw(client, map[string]interface{}{
		"model": client.Config.Model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": "Use the extract_event tool."},
			{"role": "user", "content": userPrompt},
		},
		"tools":      []map[string]interface{}{tool},
		"tool_choice": map[string]interface{}{"type": "function", "function": map[string]string{"name": "extract_event"}},
		"temperature": 0.0,
		"max_tokens":  300,
		"stream":      false,
	})
	if err != nil {
		return nil, err
	}
	raw := extractFirstToolCallArguments(resp)
	if raw == "" {
		return nil, fmt.Errorf("model did not call the extract_event tool")
	}
	fmt.Println("\n[Function call] raw arguments:")
	fmt.Println(raw)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	return validateEvent(data)
}

func extractContent(resp map[string]interface{}) string {
	choices, _ := resp["choices"].([]interface{})
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]interface{})
	message, _ := choice["message"].(map[string]interface{})
	content, _ := message["content"].(string)
	return content
}

func extractFirstToolCallArguments(resp map[string]interface{}) string {
	choices, _ := resp["choices"].([]interface{})
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]interface{})
	message, _ := choice["message"].(map[string]interface{})
	toolCalls, _ := message["tool_calls"].([]interface{})
	if len(toolCalls) == 0 {
		return ""
	}
	tc, _ := toolCalls[0].(map[string]interface{})
	fn, _ := tc["function"].(map[string]interface{})
	args, _ := fn["arguments"].(string)
	return args
}

func runMode(label string, extractor func(*openaiclient.Client) (map[string]interface{}, error), client *openaiclient.Client) map[string]interface{} {
	fmt.Printf("\n%s\nMode: %s\n%s\n", strings.Repeat("=", 40), label, strings.Repeat("=", 40))
	result, err := extractor(client)
	if err != nil {
		fmt.Printf("\n[%s] FAILED: %v\n", label, err)
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	fmt.Printf("\n[%s] parsed event:\n", label)
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
	return map[string]interface{}{"ok": true, "result": result}
}

func main() {
	client := openaiclient.NewClient()

	jsonMode := runMode("JSON mode", extractWithJSONMode, client)
	functionMode := runMode("Function calling", extractWithFunctionCall, client)

	fmt.Println("\n" + strings.Repeat("=", 40))
	fmt.Println("Summary")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("JSON mode ok: %v\n", jsonMode["ok"])
	fmt.Printf("Function call ok: %v\n", functionMode["ok"])

	if jsonMode["ok"].(bool) && functionMode["ok"].(bool) {
		j := jsonMode["result"].(map[string]interface{})
		f := functionMode["result"].(map[string]interface{})
		fmt.Println("\nBoth approaches returned valid events. Function calling gives you an explicit schema contract; JSON mode is simpler when you only need a shaped text response.")
		fmt.Printf("Names match: %v\n", j["name"] == f["name"] && j["date"] == f["date"])
	}
}
