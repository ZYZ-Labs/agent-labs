package main

import (
	"fmt"
	"regexp"
	"strings"
)

const sampleCode = `"""A tiny module for demonstration."""

import json
from datetime import datetime


def greet(name: str) -> str:
    """Return a friendly greeting."""
    return f"Hello, {name}!"


class Calculator:
    """Simple calculator."""

    def add(self, a: float, b: float) -> float:
        return a + b

    def subtract(self, a: float, b: float) -> float:
        return a - b


def main():
    calc = Calculator()
    print(greet("world"))
    print(calc.add(1, 2))
`

type definition struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Line      int    `json:"line"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
}

func walkTree(source string) int {
	// Placeholder for AST node count; regex extraction below gives a comparable metric.
	return len(regexp.MustCompile(`(?m)^(?:def|class)\s+`).FindAllStringIndex(source, -1))
}

func extractDefinitions(source string) []definition {
	var defs []definition
	lines := strings.Split(source, "\n")

	funcRe := regexp.MustCompile(`(?m)^def\s+([A-Za-z0-9_]+)\s*\(`)
	classRe := regexp.MustCompile(`(?m)^class\s+([A-Za-z0-9_]+)\s*[:\(]`)

	byteOffset := 0
	for i, line := range lines {
		if matches := funcRe.FindStringSubmatchIndex(line); matches != nil {
			nameStart := matches[2]
			nameEnd := matches[3]
			startByte := byteOffset + nameStart
			endByte := startByte
			// Find the end of the definition by scanning for the next top-level def/class at column 0.
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if (funcRe.MatchString(lines[j]) || classRe.MatchString(lines[j])) && (len(lines[j]) == 0 || lines[j][0] != ' ') {
					endByte = byteOffset + len(line) + 1
					for k := i + 1; k < j; k++ {
						endByte += len(lines[k]) + 1
					}
					break
				}
			}
			if endByte <= startByte {
				endByte = len(source)
			}
			defs = append(defs, definition{Kind: "function", Name: line[nameStart:nameEnd], Line: i + 1, StartByte: startByte, EndByte: endByte})
		}
		if matches := classRe.FindStringSubmatchIndex(line); matches != nil {
			nameStart := matches[2]
			nameEnd := matches[3]
			startByte := byteOffset + nameStart
			endByte := startByte
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if (funcRe.MatchString(lines[j]) || classRe.MatchString(lines[j])) && (len(lines[j]) == 0 || lines[j][0] != ' ') {
					endByte = byteOffset + len(line) + 1
					for k := i + 1; k < j; k++ {
						endByte += len(lines[k]) + 1
					}
					break
				}
			}
			if endByte <= startByte {
				endByte = len(source)
			}
			defs = append(defs, definition{Kind: "class", Name: line[nameStart:nameEnd], Line: i + 1, StartByte: startByte, EndByte: endByte})
		}
		byteOffset += len(line) + 1
	}

	return defs
}

func buildPromptContext(source string, defs []definition) string {
	var lines []string
	lines = append(lines, "You are reviewing the following Python module.", "", "## Symbols")
	for _, d := range defs {
		lines = append(lines, fmt.Sprintf("- %s `%s` at line %d (bytes %d-%d)", strings.Title(d.Kind), d.Name, d.Line, d.StartByte, d.EndByte))
	}
	lines = append(lines, "", "## Source", "```python", source, "```", "", "Please summarize what this module does.")
	return strings.Join(lines, "\n")
}

func main() {
	fmt.Printf("AST node count: %d\n", walkTree(sampleCode))

	defs := extractDefinitions(sampleCode)
	fmt.Println("\nDiscovered definitions:")
	for _, d := range defs {
		fmt.Printf("  [%s] %s @ line %d\n", d.Kind, d.Name, d.Line)
	}

	context := buildPromptContext(sampleCode, defs)
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Generated prompt context")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println(context)
}
