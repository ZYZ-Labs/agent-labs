package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ZYZ-Labs/agent-labs/shared/config"
)

const defaultRequirements = `Build a simple REST API for a task management service.
Users should be able to create, list, update, and delete tasks.
Each task has a title, description, status (todo/in_progress/done), and due date.
Store data in memory; no database is required.
Add basic input validation.`

func sanitizeFilename(text string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	s := re.ReplaceAllString(text, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

func generateDesignDoc(client *openaiclient.Client, requirements string) (string, error) {
	prompt := fmt.Sprintf(`You are a senior backend engineer. Write a concise design document (Markdown) for the following requirements.

Requirements:
%s

Include:
- Overview
- Endpoints (method, path, description)
- Data model
- Assumptions and constraints

Respond with Markdown only.`, requirements)
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
		MaxTokens:   intPtr(1500),
		Temperature: floatPtr(0.2),
	})
	if err != nil {
		return "", err
	}
	return client.ExtractMessage(resp).Content, nil
}

func generateAPICode(client *openaiclient.Client, requirements, designDoc string) (string, error) {
	prompt := fmt.Sprintf(`You are a senior backend engineer. Implement a runnable Go net/http application for the requirements below.

Requirements:
%s

Design document:
%s

Guidelines:
- Use only the standard library.
- Store data in memory.
- Include input validation.
- Do not include instructions or explanations outside the code.
- Output a single Go file named main.go.

Respond with the full Go source code only (no Markdown fences).`, requirements, designDoc)
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
		MaxTokens:   intPtr(2000),
		Temperature: floatPtr(0.2),
	})
	if err != nil {
		return "", err
	}
	return stripMarkdownFences(client.ExtractMessage(resp).Content), nil
}

func generateTests(client *openaiclient.Client, apiCode string) (string, error) {
	prompt := fmt.Sprintf(`You are a QA engineer. Write Go tests for the following net/http application.

Application code:
%s

Guidelines:
- Use net/http/httptest.
- Cover create, list, update, and delete endpoints.
- Include at least one validation failure test.
- Output a single Go test file named main_test.go.

Respond with the full Go test source code only (no Markdown fences).`, apiCode)
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
		MaxTokens:   intPtr(2000),
		Temperature: floatPtr(0.2),
	})
	if err != nil {
		return "", err
	}
	return stripMarkdownFences(client.ExtractMessage(resp).Content), nil
}

func generateReviewReport(client *openaiclient.Client, requirements, designDoc, apiCode, tests string) (string, error) {
	prompt := fmt.Sprintf(`You are a staff engineer reviewing the following backend artifact bundle.

Requirements:
%s

Design Document:
%s

API Code:
%s

Tests:
%s

Produce a review report (Markdown) with:
- Summary
- What was done well
- Risks and concerns
- Actionable recommendations
- Pass/needs-work verdict

Respond with Markdown only.`, requirements, designDoc, apiCode, tests)
	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
		MaxTokens:   intPtr(1500),
		Temperature: floatPtr(0.2),
	})
	if err != nil {
		return "", err
	}
	return client.ExtractMessage(resp).Content, nil
}

func stripMarkdownFences(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "```") {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "```") {
			lines = lines[:len(lines)-1]
		}
		text = strings.Join(lines, "\n")
	}
	return strings.TrimSpace(text)
}

func writeArtifact(dir, name, content string) (string, error) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func main() {
	client := openaiclient.NewClient()
	requirements := os.Getenv("REQUIREMENTS")
	if requirements == "" {
		requirements = defaultRequirements
	}
	fmt.Println("Requirements:")
	fmt.Println(requirements)
	fmt.Println()

	outputDir := filepath.Join("..", "output")
	_ = os.MkdirAll(outputDir, 0755)

	fmt.Println("Generating design doc...")
	designDoc, err := generateDesignDoc(client, requirements)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	path, _ := writeArtifact(outputDir, "design_doc.md", designDoc)
	fmt.Println("  Wrote", path)

	fmt.Println("Generating API code...")
	apiCode, err := generateAPICode(client, requirements, designDoc)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	path, _ = writeArtifact(outputDir, "api_code.go", apiCode)
	fmt.Println("  Wrote", path)

	fmt.Println("Generating tests...")
	tests, err := generateTests(client, apiCode)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	path, _ = writeArtifact(outputDir, "api_code_test.go", tests)
	fmt.Println("  Wrote", path)

	fmt.Println("Generating review report...")
	review, err := generateReviewReport(client, requirements, designDoc, apiCode, tests)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	path, _ = writeArtifact(outputDir, "review_report.md", review)
	fmt.Println("  Wrote", path)

	fmt.Println("\nCapstone artifacts generated successfully.")
}

func intPtr(v int) *int       { return &v }
func floatPtr(v float64) *float64 { return &v }
