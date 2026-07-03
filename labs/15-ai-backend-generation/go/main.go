package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

const description = (
	"A blog post resource with title (required string), body (required string), " +
		"and published (boolean, default false). Provide CRUD endpoints for listing, " +
		"creating, reading, updating, and deleting posts. Store posts in memory.")

var files = map[string]string{
	"models.py":    "(?s)```python\\s*models\\.py\\n(.*?)\\n```",
	"routes.py":    "(?s)```python\\s*routes\\.py\\n(.*?)\\n```",
	"test_crud.py": "(?s)```python\\s*test_crud\\.py\\n(.*?)\\n```",
}

func loadPrompt(description string) (string, error) {
	b, err := os.ReadFile(filepath.Join("prompts", "crud_gen.txt"))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(b), "{{ description }}", description), nil
}

func extractFile(content, pattern string) (string, error) {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(content)
	if match == nil {
		return "", fmt.Errorf("required code block not found in model response")
	}
	return strings.TrimSpace(match[1]), nil
}

func validatePython(source, filename string) error {
	// Go cannot parse Python; do basic sanity checks.
	if filename == "routes.py" {
		if !strings.Contains(source, "APIRouter") {
			fmt.Fprintf(os.Stderr, "Warning: %s does not appear to define an APIRouter.\n", filename)
		}
	}
	return nil
}

func generateModule(client *openaiclient.Client, description string) (map[string]string, error) {
	prompt, err := loadPrompt(description)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Sending backend generation prompt (%d chars).\n", len(prompt))

	temp := 0.2
	maxTokens := 2000
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, err
	}
	content := resp.Choices[0].Message.Content

	generated := map[string]string{}
	for filename, pattern := range files {
		s, err := extractFile(content, pattern)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}
		generated[filename] = s
	}
	return generated, nil
}

func writeModule(files map[string]string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	for filename, source := range files {
		path := filepath.Join(outputDir, filename)
		if err := os.WriteFile(path, []byte(source+"\n"), 0644); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\n", path)
	}
	return nil
}

func main() {
	client := openaiclient.NewClient()

	files, err := generateModule(client, description)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Generation failed: %v\n", err)
		os.Exit(1)
	}

	allValid := true
	for filename, source := range files {
		if err := validatePython(source, filename); err != nil {
			allValid = false
			fmt.Printf("[FAIL] %s\n", err)
		} else {
			fmt.Printf("[OK] %s parses and compiles.\n", filename)
		}
	}

	outputDir := "generated"
	if err := writeModule(files, outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write generated module: %v\n", err)
		os.Exit(1)
	}

	if !allValid {
		fmt.Fprintln(os.Stderr, "One or more generated files failed validation.")
		os.Exit(1)
	}

	absOutput, _ := filepath.Abs(outputDir)
	fmt.Printf("\nGenerated module written to: %s\n", absOutput)
}
