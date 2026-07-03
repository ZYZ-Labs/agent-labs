package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

func setupLogging() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
}

func logJSON(payload map[string]interface{}) {
	b, _ := json.Marshal(payload)
	log.Println(string(b))
}

func span(name, traceID string, attrs map[string]interface{}, fn func()) {
	start := time.Now()
	merged := map[string]interface{}{"event": "span.start"}
	for k, v := range attrs {
		merged[k] = v
	}
	logJSON(map[string]interface{}{
		"timestamp":  time.Now().Format(time.RFC3339Nano),
		"level":      "INFO",
		"logger":     "agent-labs.observability",
		"message":    "span.start",
		"trace_id":   traceID,
		"span_name":  name,
		"span_attrs": merged,
	})
	fn()
	durationMs := float64(time.Since(start).Nanoseconds()) / 1e6
	merged["event"] = "span.end"
	merged["duration_ms"] = durationMs
	logJSON(map[string]interface{}{
		"timestamp":  time.Now().Format(time.RFC3339Nano),
		"level":      "INFO",
		"logger":     "agent-labs.observability",
		"message":    "span.end",
		"trace_id":   traceID,
		"span_name":  name,
		"span_attrs": merged,
	})
}

func logUsage(traceID string, response *openaiclient.ChatCompletionResponse) {
	attrs := map[string]interface{}{
		"event":             "tokens.usage",
		"prompt_tokens":     response.Usage.PromptTokens,
		"completion_tokens": response.Usage.CompletionTokens,
		"total_tokens":      response.Usage.TotalTokens,
	}
	logJSON(map[string]interface{}{
		"timestamp":  time.Now().Format(time.RFC3339Nano),
		"level":      "INFO",
		"logger":     "agent-labs.observability",
		"message":    "tokens.usage",
		"trace_id":   traceID,
		"span_name":  "usage",
		"span_attrs": attrs,
	})
}

func runObservableAgent(client *openaiclient.Client, userMessage string) string {
	traceID := newUUID()
	var answer string
	span("agent.run", traceID, map[string]interface{}{"input_length": len(userMessage)}, func() {
		var response *openaiclient.ChatCompletionResponse
		span("llm.call", traceID, nil, func() {
			resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
				Messages:    []openaiclient.Message{{Role: "user", Content: userMessage}},
				MaxTokens:   intPtr(200),
				Temperature: floatPtr(0.0),
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "LLM call failed: %v\n", err)
				return
			}
			response = resp
			logUsage(traceID, resp)
		})
		if response == nil {
			return
		}
		message := response.Choices[0].Message
		answer = message.Content
		// The shared wrapper does not expose tool_calls; simulate if content looks like a tool call.
		if strings.Contains(answer, "get_weather") {
			span("tool.execute", traceID, map[string]interface{}{"tool_name": "get_weather"}, func() {})
		}
	})
	logJSON(map[string]interface{}{
		"timestamp":  time.Now().Format(time.RFC3339Nano),
		"level":      "INFO",
		"logger":     "agent-labs.observability",
		"message":    "agent.response",
		"trace_id":   traceID,
		"span_name":  "agent.run",
		"span_attrs": map[string]interface{}{"event": "agent.response", "response": answer},
	})
	return answer
}

func newUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

func main() {
	setupLogging()
	client := openaiclient.NewClient()

	question := "Explain observability in one sentence."
	fmt.Printf("User: %s\n", question)
	answer := runObservableAgent(client, question)
	fmt.Printf("Assistant: %s\n", answer)
}
