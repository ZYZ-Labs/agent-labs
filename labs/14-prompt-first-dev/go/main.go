package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

const requirement = (
	"Build an API for a task manager. Users can create a task with a title and " +
		"optional description, list all tasks, and mark a task as complete. " +
		"Store tasks in memory only.")

func loadPromptTemplate(version string) (string, error) {
	promptPath := filepath.Join("prompts", version, "spec_gen.txt")
	if _, err := os.Stat(promptPath); err != nil {
		entries, _ := os.ReadDir("prompts")
		var available []string
		for _, e := range entries {
			if e.IsDir() {
				available = append(available, e.Name())
			}
		}
		return "", fmt.Errorf("prompt template not found for version '%s'. Available versions: %v", version, available)
	}
	b, err := os.ReadFile(promptPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func renderPrompt(template, requirement string) string {
	return strings.ReplaceAll(template, "{{ requirement }}", requirement)
}

func extractCodeBlocks(content string) (string, string, error) {
	mdRe := regexp.MustCompile("(?s)```markdown\\s*\\n(.*?)\\n```")
	pyRe := regexp.MustCompile("(?s)```python\\s*\\n(.*?)\\n```")
	mdMatch := mdRe.FindStringSubmatch(content)
	pyMatch := pyRe.FindStringSubmatch(content)
	if mdMatch == nil || pyMatch == nil {
		return "", "", fmt.Errorf("model response did not contain both required markdown and python code blocks")
	}
	return strings.TrimSpace(mdMatch[1]), strings.TrimSpace(pyMatch[1]), nil
}

func generateArtifacts(client *openaiclient.Client, requirement, version string) (string, string, error) {
	template, err := loadPromptTemplate(version)
	if err != nil {
		return "", "", err
	}
	prompt := renderPrompt(template, requirement)

	fmt.Printf("Using prompt version: %s\n", version)
	fmt.Printf("Sending %d prompt characters to model.\n", len(prompt))

	temp := 0.2
	maxTokens := 1500
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return "", "", err
	}
	content := resp.Choices[0].Message.Content
	return extractCodeBlocks(content)
}

func writeArtifacts(spec, scaffold string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	specPath := filepath.Join(outputDir, "api_spec.md")
	scaffoldPath := filepath.Join(outputDir, "scaffold.py")
	if err := os.WriteFile(specPath, []byte(spec+"\n"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(scaffoldPath, []byte(scaffold+"\n"), 0644); err != nil {
		return err
	}
	fmt.Printf("Wrote artifacts to %s\n", outputDir)
	fmt.Printf("  - %s\n", filepath.Base(specPath))
	fmt.Printf("  - %s\n", filepath.Base(scaffoldPath))
	return nil
}

func preview(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "\n..."
}

func main() {
	client := openaiclient.NewClient()
	version := os.Getenv("PROMPT_VERSION")
	if version == "" {
		version = "v2"
	}

	spec, scaffold, err := generateArtifacts(client, requirement, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate artifacts: %v\n", err)
		os.Exit(1)
	}

	outputDir := "generated"
	if err := writeArtifacts(spec, scaffold, outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write artifacts: %v\n", err)
		os.Exit(1)
	}

	absOutput, _ := filepath.Abs(outputDir)
	fmt.Println("\n=== Generated API Spec Preview ===")
	fmt.Println(preview(spec, 400))
	fmt.Println("\n=== Generated Scaffold Preview ===")
	fmt.Println(preview(scaffold, 400))
	fmt.Printf("\nArtifacts written to: %s\n", absOutput)
}
