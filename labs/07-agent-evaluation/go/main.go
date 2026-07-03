package main

import (
	"encoding/json"
	"fmt"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

type testCase struct {
	Input             string   `json:"input"`
	ExpectedKeywords  []string `json:"expected_keywords"`
	Reference         string   `json:"reference"`
}

var testCases = []testCase{
	{
		Input:            "What port range is safe for user services?",
		ExpectedKeywords: []string{"1024", "65535"},
		Reference:        "User services should use ports from 1024 to 65535.",
	},
	{
		Input:            "Explain idempotency in one sentence.",
		ExpectedKeywords: []string{"same", "multiple", "result"},
		Reference:        "Idempotency means calling an operation multiple times produces the same result.",
	},
}

func ruleCheck(answer string, keywords []string) map[string]interface{} {
	var missing []string
	lower := strings.ToLower(answer)
	for _, kw := range keywords {
		if !strings.Contains(lower, strings.ToLower(kw)) {
			missing = append(missing, kw)
		}
	}
	return map[string]interface{}{"passed": len(missing) == 0, "missing": missing}
}

func llmJudge(client *openaiclient.Client, answer, reference string) map[string]interface{} {
	prompt := fmt.Sprintf(`Rate how well the following answer matches the reference answer.
Answer: %s
Reference: %s
Respond with JSON only: {"score": 1-10, "reason": "..."}`, answer, reference)
	temp := 0.0
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages: []openaiclient.Message{{Role: "user", Content: prompt}},
		ResponseFormat: map[string]string{"type": "json_object"},
		MaxTokens: intPtr(200),
		Temperature: &temp,
	})
	if err != nil {
		return map[string]interface{}{"score": 0, "reason": fmt.Sprintf("judge call failed: %v", err)}
	}
	var result map[string]interface{}
	_ = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result)
	return result
}

func evaluateAgent(client *openaiclient.Client) []map[string]interface{} {
	var results []map[string]interface{}
	for _, c := range testCases {
		answer := simpleAgent(client, c.Input)
		ruleResult := ruleCheck(answer, c.ExpectedKeywords)
		judgeResult := llmJudge(client, answer, c.Reference)
		results = append(results, map[string]interface{}{
			"input":            c.Input,
			"answer":           answer,
			"rule_passed":      ruleResult["passed"],
			"missing_keywords": ruleResult["missing"],
			"judge_score":      judgeResult["score"],
			"judge_reason":     judgeResult["reason"],
		})
	}
	return results
}

func simpleAgent(client *openaiclient.Client, question string) string {
	temp := 0.0
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages: []openaiclient.Message{{Role: "user", Content: question}},
		MaxTokens: intPtr(200),
		Temperature: &temp,
	})
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return resp.Choices[0].Message.Content
}

func intPtr(i int) *int {
	return &i
}

func main() {
	client := openaiclient.NewClient()
	results := evaluateAgent(client)

	passed := 0
	total := len(results)
	var scoreSum float64
	for _, r := range results {
		if r["rule_passed"].(bool) {
			passed++
		}
		if s, ok := r["judge_score"].(float64); ok {
			scoreSum += s
		}
	}
	avg := scoreSum / float64(total)

	b, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(b))
	fmt.Printf("\nRule checks passed: %d/%d\n", passed, total)
	fmt.Printf("Average judge score: %.1f/10\n", avg)
}
