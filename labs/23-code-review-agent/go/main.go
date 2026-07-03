package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

func findRuff() string {
	if p, err := exec.LookPath("ruff"); err == nil {
		return p
	}
	return ""
}

func runSyntaxCheck(path string) map[string]interface{} {
	cmd := exec.Command("python", "-m", "py_compile", path)
	if err := cmd.Run(); err != nil {
		return map[string]interface{}{"ok": false, "issues": []string{fmt.Sprintf("Syntax error: %v", err)}}
	}
	return map[string]interface{}{"ok": true, "issues": []string{}}
}

func runRuff(path string) map[string]interface{} {
	ruffBin := findRuff()
	if ruffBin == "" {
		log.Println("ruff not installed; skipping style check")
		return map[string]interface{}{"ok": true, "issues": []string{"ruff not installed"}}
	}
	cmd := exec.Command(ruffBin, "check", path, "--output-format", "json")
	out, _ := cmd.Output()
	issues := []string{}
	if len(out) > 0 {
		var parsed []map[string]interface{}
		if err := json.Unmarshal(out, &parsed); err == nil {
			for _, item := range parsed {
				loc, _ := item["location"].(map[string]interface{})
				row, _ := loc["row"].(float64)
				code, _ := item["code"].(string)
				message, _ := item["message"].(string)
				issues = append(issues, fmt.Sprintf("Line %d: %s - %s", int(row), code, message))
			}
		} else {
			issues = append(issues, strings.TrimSpace(string(out)))
		}
	}
	return map[string]interface{}{"ok": len(issues) == 0, "issues": issues}
}

func llmReview(path, source string, checks map[string]interface{}, client *openaiclient.Client) string {
	if client == nil {
		return "LLM review skipped (no API key). Manual review recommended."
	}
	syntax, _ := checks["syntax"].(map[string]interface{})
	ruff, _ := checks["ruff"].(map[string]interface{})
	syntaxOK, _ := syntax["ok"].(bool)
	ruffIssues, _ := ruff["issues"].([]interface{})
	var issueList []string
	for _, i := range ruffIssues {
		issueList = append(issueList, i.(string))
	}

	prompt := fmt.Sprintf(`Review the following Python file for style, bugs, and maintainability.

File: %s

Static checks:
- Syntax OK: %v
- Style issues: %v

Source code:
```python
%s
```

Provide a concise review with:
1. Critical issues
2. Suggestions
3. Overall verdict (OK / Needs work).`, filepath.Base(path), syntaxOK, issueList, source)

	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
		Temperature: floatPtr(0.2),
		MaxTokens:   intPtr(400),
	})
	if err != nil {
		return fmt.Sprintf("LLM review failed: %v", err)
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

func reviewFile(path string, client *openaiclient.Client) map[string]interface{} {
	log.Printf("Reviewing %s", path)
	sourceBytes, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{"file": path, "error": err.Error()}
	}
	source := string(sourceBytes)
	checks := map[string]interface{}{
		"syntax": runSyntaxCheck(path),
		"ruff":   runRuff(path),
	}
	review := llmReview(path, source, checks, client)

	ruffIssues := []string{}
	if ruff, ok := checks["ruff"].(map[string]interface{}); ok {
		for _, i := range ruff["issues"].([]interface{}) {
			if s := i.(string); s != "ruff not installed" {
				ruffIssues = append(ruffIssues, s)
			}
		}
	}
	syntaxOK, _ := checks["syntax"].(map[string]interface{})["ok"].(bool)
	verdict := "ok"
	if !syntaxOK || len(ruffIssues) > 0 {
		verdict = "needs_work"
	}

	return map[string]interface{}{
		"file":       path,
		"checks":     checks,
		"llm_review": review,
		"verdict":    verdict,
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func main() {
	cfg := openaiclient.LoadConfig()
	var client *openaiclient.Client
	if cfg.APIKey != "" || strings.HasPrefix(cfg.BaseURL, "http://localhost") {
		client = openaiclient.NewClient()
	} else {
		log.Println("LLM client disabled: OPENAI_API_KEY not set")
	}

	target := "sample_code.py"
	if len(os.Args) > 1 {
		target = os.Args[1]
	}
	if _, err := os.Stat(target); err != nil {
		log.Fatalf("File not found: %s", target)
	}

	report := reviewFile(target, client)
	b, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(b))
}
