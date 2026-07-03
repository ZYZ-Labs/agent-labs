package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

type testCase struct {
	ID              string   `json:"id"`
	Input           string   `json:"input"`
	Reference       string   `json:"reference"`
	ExpectedKeywords []string `json:"expected_keywords"`
	ExpectJSON      bool     `json:"expect_json"`
	MaxLatencyMs    *int     `json:"max_latency_ms"`
}

func loadTestCases(path string) ([]testCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []testCase
	if err := json.Unmarshal(b, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

func ruleCheck(answer string, c testCase, latencyMs float64) map[string]interface{} {
	checks := map[string]interface{}{}

	var missing []string
	lower := strings.ToLower(answer)
	for _, kw := range c.ExpectedKeywords {
		if !strings.Contains(lower, strings.ToLower(kw)) {
			missing = append(missing, kw)
		}
	}
	checks["keywords"] = map[string]interface{}{"passed": len(missing) == 0, "missing": missing}

	if c.ExpectJSON {
		if err := json.Unmarshal([]byte(answer), new(interface{})); err != nil {
			checks["json"] = map[string]interface{}{"passed": false, "error": "not valid JSON"}
		} else {
			checks["json"] = map[string]interface{}{"passed": true}
		}
	}

	if c.MaxLatencyMs != nil {
		checks["latency"] = map[string]interface{}{"passed": latencyMs <= float64(*c.MaxLatencyMs), "latency_ms": latencyMs}
	}

	allPassed := true
	for _, v := range checks {
		if m, ok := v.(map[string]interface{}); ok {
			if passed, ok := m["passed"].(bool); ok && !passed {
				allPassed = false
			}
		}
	}
	return map[string]interface{}{"passed": allPassed, "checks": checks}
}

func llmJudge(client *openaiclient.Client, answer, reference string) map[string]interface{} {
	prompt := fmt.Sprintf(`Rate how well the following answer matches the reference answer.
Answer: %s
Reference: %s
Respond with JSON only: {"score": 1-10, "reason": "..."}`, answer, reference)
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:       []openaiclient.Message{{Role: "user", Content: prompt}},
		ResponseFormat: map[string]string{"type": "json_object"},
		MaxTokens:      intPtr(200),
		Temperature:    floatPtr(0.0),
	})
	if err != nil {
		return map[string]interface{}{"score": 0, "reason": fmt.Sprintf("judge call failed: %v", err)}
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return map[string]interface{}{"score": 0, "reason": fmt.Sprintf("failed to parse judge response: %v", err)}
	}
	return result
}

func runAgent(client *openaiclient.Client, question string) string {
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: question}},
		MaxTokens:   intPtr(200),
		Temperature: floatPtr(0.0),
	})
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return resp.Choices[0].Message.Content
}

func evaluate(client *openaiclient.Client, cases []testCase) map[string]interface{} {
	var results []map[string]interface{}
	for _, c := range cases {
		start := time.Now()
		answer := runAgent(client, c.Input)
		latencyMs := float64(time.Since(start).Nanoseconds()) / 1e6

		ruleResult := ruleCheck(answer, c, latencyMs)
		judgeResult := llmJudge(client, answer, c.Reference)

		results = append(results, map[string]interface{}{
			"id":           c.ID,
			"input":        c.Input,
			"answer":       answer,
			"rule_passed":  ruleResult["passed"],
			"rule_details": ruleResult["checks"],
			"judge_score":  judgeResult["score"],
			"judge_reason": judgeResult["reason"],
		})
	}

	total := len(results)
	rulePassed := 0
	var scoreSum float64
	var scoreCount int
	for _, r := range results {
		if r["rule_passed"].(bool) {
			rulePassed++
		}
		if s, ok := r["judge_score"].(float64); ok {
			scoreSum += s
			scoreCount++
		}
	}
	avg := 0.0
	if scoreCount > 0 {
		avg = scoreSum / float64(scoreCount)
	}

	return map[string]interface{}{
		"total":               total,
		"rule_pass_rate":      float64(rulePassed) / float64(total),
		"average_judge_score": avg,
		"cases":               results,
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func main() {
	client := openaiclient.NewClient()
	cases, err := loadTestCases("../python/test_cases.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load test cases: %v\n", err)
		os.Exit(1)
	}
	report := evaluate(client, cases)
	b, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(b))
	fmt.Printf("\nRule pass rate: %.0f%%\n", report["rule_pass_rate"].(float64)*100)
	fmt.Printf("Average judge score: %.1f/10\n", report["average_judge_score"].(float64))
}
