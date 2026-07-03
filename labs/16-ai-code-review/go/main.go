package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

const reviewPrompt = `You are a meticulous code reviewer. Review the following Python source file and return ONLY a JSON object.

Rules for the JSON object:
- Top-level keys must be exactly: "security", "style", "logic".
- Each value is a list of findings. An empty list is allowed.
- Each finding is an object with these keys:
  - "severity": one of "HIGH", "MEDIUM", "LOW".
  - "line": integer line number, or null if not applicable.
  - "message": concise description of the issue.
  - "suggestion": concrete recommendation to fix it.

Be strict but fair. Focus on real issues, not nitpicks.

```python
%s
```
`

var categories = []string{"security", "style", "logic"}
var severityOrder = map[string]int{"HIGH": 0, "MEDIUM": 1, "LOW": 2}

func readSourceFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("source file not found: %s", path)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is not a file: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func reviewCode(client *openaiclient.Client, source string) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(reviewPrompt, source)
	fmt.Printf("Sending code review prompt (%d chars of code).\n", len(source))

	temp := 0.2
	maxTokens := 1200
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:       []openaiclient.Message{{Role: "user", Content: prompt}},
		Temperature:    &temp,
		MaxTokens:      &maxTokens,
		ResponseFormat: map[string]string{"type": "json_object"},
	})
	if err != nil {
		return nil, err
	}
	content := resp.Choices[0].Message.Content
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(content), &report); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %v\nRaw content:\n%s", err, content)
	}
	return report, nil
}

func validateReport(report map[string]interface{}) error {
	if report == nil {
		return fmt.Errorf("review report is not a JSON object")
	}
	var missing []string
	for _, cat := range categories {
		if _, ok := report[cat]; !ok {
			missing = append(missing, cat)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("review report missing required categories: %v", missing)
	}
	for _, cat := range categories {
		findings, ok := report[cat].([]interface{})
		if !ok {
			return fmt.Errorf("category '%s' must be a list", cat)
		}
		for idx, f := range findings {
			finding, ok := f.(map[string]interface{})
			if !ok {
				return fmt.Errorf("finding %d in '%s' is not an object", idx, cat)
			}
			for _, key := range []string{"severity", "message", "suggestion"} {
				if _, ok := finding[key]; !ok {
					return fmt.Errorf("finding %d in '%s' missing key '%s'", idx, cat, key)
				}
			}
			sev, ok := finding["severity"].(string)
			if !ok || severityOrder[sev] == 0 && sev != "HIGH" {
				if _, ok2 := severityOrder[sev]; !ok2 {
					return fmt.Errorf("finding %d in '%s' has invalid severity '%v'", idx, cat, finding["severity"])
				}
			}
		}
	}
	return nil
}

func printReport(filePath string, lineCount int, report map[string]interface{}) {
	var categoriesPresent []string
	for _, cat := range categories {
		if findings, ok := report[cat].([]interface{}); ok && len(findings) > 0 {
			categoriesPresent = append(categoriesPresent, cat)
		}
	}

	fmt.Printf("\nReview: %s\n", filePath)
	fmt.Printf("Lines: %d\n", lineCount)
	if len(categoriesPresent) == 0 {
		fmt.Println("Categories: none with findings")
	} else {
		fmt.Printf("Categories: %s\n", strings.Join(categoriesPresent, ", "))
	}

	total := 0
	counts := map[string]int{"HIGH": 0, "MEDIUM": 0, "LOW": 0}

	for _, cat := range categories {
		findings, _ := report[cat].([]interface{})
		if len(findings) == 0 {
			continue
		}
		// Sort by severity.
		sorted := make([]map[string]interface{}, len(findings))
		for i, f := range findings {
			sorted[i] = f.(map[string]interface{})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return severityOrder[sorted[i]["severity"].(string)] < severityOrder[sorted[j]["severity"].(string)]
		})

		fmt.Printf("\n%s\n", strings.ToUpper(cat))
		for _, finding := range sorted {
			total++
			sev := finding["severity"].(string)
			counts[sev]++
			lineInfo := ""
			if line, ok := finding["line"]; ok && line != nil {
				lineInfo = fmt.Sprintf(" at line %v", line)
			}
			fmt.Printf("  - %s%s: %s\n", sev, lineInfo, finding["message"])
			fmt.Printf("    Suggestion: %s\n", finding["suggestion"])
		}
	}

	fmt.Printf("\nSummary: %d issue(s) found. %d high, %d medium, %d low.\n", total, counts["HIGH"], counts["MEDIUM"], counts["LOW"])
}

func main() {
	client := openaiclient.NewClient()

	target := "sample_code.py"
	if len(os.Args) > 1 {
		target = os.Args[1]
	}
	absTarget, _ := filepath.Abs(target)

	source, err := readSourceFile(absTarget)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report, err := reviewCode(client, source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Review failed: %v\n", err)
		os.Exit(1)
	}

	if err := validateReport(report); err != nil {
		fmt.Fprintf(os.Stderr, "Review failed: %v\n", err)
		os.Exit(1)
	}

	printReport(absTarget, len(strings.Split(source, "\n")), report)
}
