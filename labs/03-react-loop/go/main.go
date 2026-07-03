package main

import (
	"fmt"
	"regexp"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

const systemPrompt = `You are a helpful assistant that solves problems step by step.
You must follow this format exactly:

Thought: describe your reasoning
Action: tool_name(arg1, arg2, ...)
Observation: the result of the action (provided by the system)
...
Final Answer: the final answer

Available tools:
- calculator(expression: str) - evaluates a Python arithmetic expression safely
- finish(answer: str) - use when you have the final answer
`

func calculator(expression string) string {
	allowed := "0123456789+-*/(). "
	for _, c := range expression {
		if !strings.ContainsRune(allowed, c) {
			return "Error: invalid characters"
		}
	}
	// Use a tiny recursive descent parser for safe arithmetic evaluation.
	result, err := evalExpression(strings.TrimSpace(expression))
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("%v", result)
}

// --- minimal safe arithmetic evaluator ---

type parser struct {
	s string
	i int
}

func (p *parser) peek() byte {
	if p.i >= len(p.s) {
		return 0
	}
	return p.s[p.i]
}

func (p *parser) next() byte {
	ch := p.peek()
	if p.i < len(p.s) {
		p.i++
	}
	return ch
}

func (p *parser) skipSpaces() {
	for p.peek() == ' ' {
		p.next()
	}
}

func evalExpression(s string) (float64, error) {
	p := &parser{s: s}
	return p.parseExpr()
}

func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		ch := p.peek()
		if ch != '+' && ch != '-' {
			break
		}
		p.next()
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if ch == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		ch := p.peek()
		if ch != '*' && ch != '/' {
			break
		}
		p.next()
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if ch == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *parser) parseFactor() (float64, error) {
	p.skipSpaces()
	ch := p.peek()
	if ch == '(' {
		p.next()
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.peek() != ')' {
			return 0, fmt.Errorf("expected ')'")
		}
		p.next()
		return val, nil
	}
	return p.parseNumber()
}

func (p *parser) parseNumber() (float64, error) {
	p.skipSpaces()
	start := p.i
	hasDot := false
	for {
		ch := p.peek()
		if ch >= '0' && ch <= '9' {
			p.next()
		} else if ch == '.' && !hasDot {
			hasDot = true
			p.next()
		} else {
			break
		}
	}
	if start == p.i {
		return 0, fmt.Errorf("expected number at position %d", p.i)
	}
	var val float64
	_, err := fmt.Sscanf(p.s[start:p.i], "%f", &val)
	if err != nil {
		return 0, err
	}
	return val, nil
}

var actionRe = regexp.MustCompile(`Action:\s*(\w+)\((.*)\)`)

func parseAction(text string) (string, []string) {
	match := actionRe.FindStringSubmatch(text)
	if match == nil {
		return "", nil
	}
	toolName := match[1]
	argsStr := match[2]
	parts := strings.Split(argsStr, ",")
	var args []string
	for _, a := range parts {
		a = strings.TrimSpace(a)
		a = strings.Trim(a, `"`)
		a = strings.Trim(a, `'`) //nolint:gocritic // mirror Python behavior
		if a != "" {
			args = append(args, a)
		}
	}
	return toolName, args
}

func runReact(client *openaiclient.Client, question string, maxSteps int) string {
	messages := []openaiclient.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}

	temp := 0.0
	maxTokens := 200

	for step := 0; step < maxSteps; step++ {
		resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages:    messages,
			Temperature: &temp,
			MaxTokens:   &maxTokens,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return "Error during chat completion"
		}
		text := resp.Choices[0].Message.Content
		fmt.Printf("\n--- Step %d ---\n%s\n", step+1, text)

		if strings.Contains(text, "Final Answer:") {
			return strings.TrimSpace(strings.SplitN(text, "Final Answer:", 2)[1])
		}

		toolName, args := parseAction(text)
		var observation string
		if toolName == "" {
			observation = "Observation: I did not understand the action. Please use 'Action: tool_name(args)'."
		} else if toolName == "calculator" && len(args) > 0 {
			result := calculator(args[0])
			observation = fmt.Sprintf("Observation: %s", result)
		} else if toolName == "finish" && len(args) > 0 {
			return args[0]
		} else {
			observation = fmt.Sprintf("Observation: unknown tool '%s'", toolName)
		}

		fmt.Println(observation)
		messages = append(messages, openaiclient.Message{Role: "assistant", Content: text})
		messages = append(messages, openaiclient.Message{Role: "user", Content: observation})
	}

	return "Reached max steps without final answer."
}

func main() {
	client := openaiclient.NewClient()
	question := "What is (128 + 256) * 2 - 100?"
	fmt.Println("Question:", question)
	answer := runReact(client, question, 10)
	fmt.Println("\nFinal Answer:", answer)
}
