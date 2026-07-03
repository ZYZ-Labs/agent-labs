package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

var allowedCommands = []string{
	"systemctl restart",
	"kubectl rollout restart",
	"docker restart",
	"echo",
	"df",
}

func parseLogs(path string) []map[string]string {
	pattern := regexp.MustCompile(`^(\S+)\s+(\w+)\s+(.+)$`)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []map[string]string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		match := pattern.FindStringSubmatch(line)
		if match != nil {
			entries = append(entries, map[string]string{"ts": match[1], "level": match[2], "message": match[3]})
		} else {
			entries = append(entries, map[string]string{"ts": "", "level": "UNKNOWN", "message": line})
		}
	}
	return entries
}

func summarizeErrors(entries []map[string]string) map[string]interface{} {
	var errorMessages []string
	counts := map[string]int{}
	for _, e := range entries {
		counts[e["level"]]++
		if e["level"] == "ERROR" || e["level"] == "FATAL" {
			errorMessages = append(errorMessages, e["message"])
		}
	}
	return map[string]interface{}{"error_messages": errorMessages, "counts": counts}
}

func diagnose(entries []map[string]string, client *openaiclient.Client) map[string]interface{} {
	if client == nil {
		return map[string]interface{}{
			"diagnosis": "Database appears unreachable (multiple connection timeouts and 503 errors). Disk usage is also elevated.",
			"commands": []string{
				"echo 'Checking database service status...'",
				"systemctl restart postgresql",
				"echo 'Monitoring disk usage...'",
				"df -h",
			},
			"risk": "medium",
		}
	}

	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%s: %s", e["level"], e["message"]))
	}
	logText := strings.Join(lines, "\n")
	prompt := fmt.Sprintf(`You are an SRE agent. Diagnose the following logs and respond with JSON in this exact shape:
{
  "diagnosis": "short diagnosis",
  "commands": ["command1", "command2"],
  "risk": "low|medium|high"
}

Logs:
%s`, logText)

	resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
		Messages:       []openaiclient.Message{{Role: "user", Content: prompt}},
		Temperature:    floatPtr(0.2),
		MaxTokens:      intPtr(200),
		ResponseFormat: map[string]string{"type": "json_object"},
	})
	if err != nil {
		return map[string]interface{}{"diagnosis": fmt.Sprintf("Diagnosis failed: %v", err), "commands": []string{}, "risk": "unknown"}
	}
	content := resp.Choices[0].Message.Content
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return map[string]interface{}{"diagnosis": content, "commands": []string{}, "risk": "unknown"}
	}
	return parsed
}

func isCommandAllowed(command string) bool {
	for _, prefix := range allowedCommands {
		if strings.HasPrefix(strings.TrimSpace(command), prefix) {
			return true
		}
	}
	return false
}

func executeCommands(commands []string) []map[string]interface{} {
	var results []map[string]interface{}
	for _, cmd := range commands {
		fmt.Printf("  $ %s\n", cmd)
		if !isCommandAllowed(cmd) {
			fmt.Println("    -> SKIPPED (not in allowlist)")
			results = append(results, map[string]interface{}{"command": cmd, "status": "skipped", "output": ""})
			continue
		}
		result := exec.Command("sh", "-c", cmd)
		out, err := result.CombinedOutput()
		output := strings.TrimSpace(string(out))
		if output == "" {
			output = "<no output>"
		}
		if err != nil {
			fmt.Printf("    -> ERROR: %v\n", err)
			results = append(results, map[string]interface{}{"command": cmd, "status": "error", "output": err.Error()})
			continue
		}
		fmt.Printf("    -> exit 0\n")
		results = append(results, map[string]interface{}{"command": cmd, "status": "executed", "output": output})
	}
	return results
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

	logfile := "sample_app.log"
	if len(os.Args) > 1 {
		logfile = os.Args[1]
	}
	if _, err := os.Stat(logfile); err != nil {
		log.Fatalf("Log file not found: %s", logfile)
	}

	entries := parseLogs(logfile)
	fmt.Printf("Read %d log entries from %s\n", len(entries), logfile)
	diagnosis := diagnose(entries, client)

	fmt.Println("\n=== Diagnosis ===")
	fmt.Println(diagnosis["diagnosis"])
	fmt.Printf("Risk: %v\n", diagnosis["risk"])

	commands := []string{}
	if c, ok := diagnosis["commands"].([]interface{}); ok {
		for _, cmd := range c {
			commands = append(commands, cmd.(string))
		}
	}
	fmt.Println("\n=== Proposed remediation commands ===")
	for _, cmd := range commands {
		fmt.Printf("  - %s\n", cmd)
	}
	if len(commands) == 0 {
		fmt.Println("No remediation commands proposed.")
		return
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nExecute proposed commands? [y/n]: ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Execution cancelled by operator.")
		return
	}

	fmt.Println("\nExecuting commands...")
	results := executeCommands(commands)
	fmt.Println("\nExecution summary:")
	b, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(b))
}
