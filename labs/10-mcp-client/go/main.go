package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type mcpStdioClient struct {
	script      string
	process     *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Reader
	stderr      io.ReadCloser
	requestID   int
}

func defaultServerScript() string {
	return filepath.Join("..", "..", "09-mcp-server-stdio", "go")
}

func newMCPStdioClient(serverDir string) *mcpStdioClient {
	if serverDir == "" {
		serverDir = os.Getenv("MCP_SERVER_DIR")
	}
	if serverDir == "" {
		serverDir = defaultServerScript()
	}
	return &mcpStdioClient{script: serverDir}
}

func (c *mcpStdioClient) connect() error {
	fmt.Printf("Starting MCP server: %s\n", c.script)
	c.process = exec.Command("go", "run", "main.go")
	c.process.Dir = c.script
	stdin, err := c.process.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := c.process.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := c.process.StderrPipe()
	if err != nil {
		return err
	}
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)
	c.stderr = stderr
	return c.process.Start()
}

func (c *mcpStdioClient) disconnect() {
	if c.process != nil && c.process.Process != nil {
		_ = c.stdin.Close()
		_ = c.process.Process.Kill()
		_ = c.process.Wait()
	}
}

func (c *mcpStdioClient) nextID() int {
	c.requestID++
	return c.requestID
}

func (c *mcpStdioClient) send(message map[string]interface{}) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	data := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	_, err = c.stdin.Write([]byte(data))
	if err != nil {
		return err
	}
	return nil
}

func (c *mcpStdioClient) recv() (map[string]interface{}, error) {
	headers := map[string]string{}
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			key := strings.ToLower(strings.TrimSpace(line[:idx]))
			value := strings.TrimSpace(line[idx+1:])
			headers[key] = value
		}
	}
	length, _ := strconv.Atoi(headers["content-length"])
	if length == 0 {
		return nil, fmt.Errorf("empty message body")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(c.stdout, body)
	if err != nil {
		return nil, err
	}
	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *mcpStdioClient) initialize() (map[string]interface{}, error) {
	if err := c.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.nextID(),
		"method":  "initialize",
		"params":  map[string]interface{}{"protocolVersion": "2024-11-05", "capabilities": map[string]interface{}{}},
	}); err != nil {
		return nil, err
	}
	result, err := c.recv()
	if err != nil {
		return nil, err
	}
	_ = c.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      nil,
		"method":  "notifications/initialized",
	})
	return result, nil
}

func (c *mcpStdioClient) listTools() ([]map[string]interface{}, error) {
	if err := c.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.nextID(),
		"method":  "tools/list",
	}); err != nil {
		return nil, err
	}
	resp, err := c.recv()
	if err != nil {
		return nil, err
	}
	if _, ok := resp["error"]; ok {
		return nil, fmt.Errorf("%v", resp["error"])
	}
	res, _ := resp["result"].(map[string]interface{})
	tools, _ := res["tools"].([]interface{})
	var out []map[string]interface{}
	for _, t := range tools {
		if m, ok := t.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func (c *mcpStdioClient) callTool(name string, arguments map[string]interface{}) (map[string]interface{}, error) {
	if err := c.send(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.nextID(),
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": name, "arguments": arguments},
	}); err != nil {
		return nil, err
	}
	resp, err := c.recv()
	if err != nil {
		return nil, err
	}
	if _, ok := resp["error"]; ok {
		return nil, fmt.Errorf("%v", resp["error"])
	}
	res, _ := resp["result"].(map[string]interface{})
	return res, nil
}

func parseSSE(lines []string) []map[string]string {
	var events []map[string]string
	var event map[string]string
	for _, raw := range lines {
		line := strings.TrimRight(strings.TrimRight(raw, "\n"), "\r")
		if line == "" {
			if len(event) > 0 {
				events = append(events, event)
				event = nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			key := line[:idx]
			value := strings.TrimLeft(line[idx+1:], " ")
			if event == nil {
				event = map[string]string{}
			}
			event[key] = value
		}
	}
	if len(event) > 0 {
		events = append(events, event)
	}
	return events
}

func demoSSE() {
	rawStream := ":heartbeat\n\nevent: message\ndata: {" +
		`"tool": "calculate", "args": {"expression": "1+1"}}` + "\n\nevent: status\ndata: processing\n\nevent: done\ndata: finished\n\n"
	lines := strings.Split(rawStream, "\n")
	fmt.Println("\n[SSE transport concept]")
	for _, event := range parseSSE(lines) {
		fmt.Println("  SSE event:", event)
	}
}

func main() {
	client := newMCPStdioClient("")
	if err := client.connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Client failed: %v\n", err)
		os.Exit(1)
	}
	defer client.disconnect()

	initResponse, err := client.initialize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Client failed: %v\n", err)
		os.Exit(1)
	}
	result, _ := initResponse["result"].(map[string]interface{})
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("[initialize]", string(b))

	tools, err := client.listTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Client failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n[tools]")
	for _, tool := range tools {
		fmt.Printf("  - %s: %v\n", tool["name"], tool["description"])
	}

	res, err := client.callTool("calculate", map[string]interface{}{"expression": "(10 + 5) / 3"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Client failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n[tools/call calculate]")
	content, _ := res["content"].([]interface{})
	for _, item := range content {
		if m, ok := item.(map[string]interface{}); ok {
			fmt.Println(" ", m["text"])
		}
	}

	demoSSE()
}
