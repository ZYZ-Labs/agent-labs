package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var logger = log.New(os.Stderr, "mcp-server: ", log.LstdFlags)

// ---------- safe calculator ----------

type calcParser struct {
	s string
	i int
}

func (p *calcParser) peek() byte {
	if p.i >= len(p.s) {
		return 0
	}
	return p.s[p.i]
}

func (p *calcParser) next() byte {
	ch := p.peek()
	if p.i < len(p.s) {
		p.i++
	}
	return ch
}

func (p *calcParser) skipSpaces() {
	for p.peek() == ' ' {
		p.next()
	}
}

func safeEvalExpression(s string) (float64, error) {
	p := &calcParser{s: s}
	return p.parseExpr()
}

func (p *calcParser) parseExpr() (float64, error) {
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

func (p *calcParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		ch := p.peek()
		if ch != '*' && ch != '/' && ch != '%' {
			break
		}
		p.next()
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		switch ch {
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case '%':
			left = float64(int64(left) % int64(right))
		}
	}
	return left, nil
}

func (p *calcParser) parseFactor() (float64, error) {
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
	if ch == '+' || ch == '-' {
		p.next()
		val, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if ch == '-' {
			return -val, nil
		}
		return val, nil
	}
	return p.parseNumber()
}

func (p *calcParser) parseNumber() (float64, error) {
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
		return 0, fmt.Errorf("expected number")
	}
	return strconv.ParseFloat(p.s[start:p.i], 64)
}

func calculate(expression string) map[string]interface{} {
	val, err := safeEvalExpression(strings.TrimSpace(expression))
	if err != nil {
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": fmt.Sprintf("Invalid expression: %v", err)}},
			"isError": true,
		}
	}
	return map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": fmt.Sprintf("%v", val)}},
		"isError": false,
	}
}

var tools = []map[string]interface{}{
	{
		"name":        "get_current_time",
		"description": "Return the current time in ISO 8601 format.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "IANA timezone name, e.g. UTC or Asia/Shanghai.",
				},
			},
		},
	},
	{
		"name":        "calculate",
		"description": "Evaluate a simple arithmetic expression.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "Arithmetic expression with +, -, *, /, parentheses, numbers.",
				},
			},
			"required": []string{"expression"},
		},
	},
}

var toolHandlers = map[string]func(map[string]interface{}) map[string]interface{}{
	"get_current_time": func(args map[string]interface{}) map[string]interface{} {
		tzName := "UTC"
		if v, ok := args["timezone"].(string); ok && v != "" {
			tzName = v
		}
		loc, err := time.LoadLocation(tzName)
		if err != nil {
			return map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": fmt.Sprintf("Unsupported timezone: %s. Using UTC.", tzName)}},
				"isError": true,
			}
		}
		return map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": time.Now().In(loc).Format(time.RFC3339)}},
			"isError": false,
		}
	},
	"calculate": func(args map[string]interface{}) map[string]interface{} {
		expr, _ := args["expression"].(string)
		return calculate(expr)
	},
}

// ---------- JSON-RPC framing ----------

func readMessage(r *bufio.Reader) (map[string]interface{}, error) {
	headers := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(headers) == 0 {
				return nil, io.EOF
			}
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
		return nil, nil
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	if err != nil {
		return nil, err
	}
	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func writeMessage(w io.Writer, msg map[string]interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

// ---------- request handlers ----------

func makeResponse(requestID interface{}, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result":  result,
	}
}

func makeError(requestID interface{}, code int, message string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"error":   map[string]interface{}{"code": code, "message": message},
	}
}

func handleRequest(request map[string]interface{}) map[string]interface{} {
	method, _ := request["method"].(string)
	params, _ := request["params"].(map[string]interface{})
	reqID := request["id"]

	logger.Printf("Received %s", method)

	switch method {
	case "initialize":
		return makeResponse(reqID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "agent-labs-mcp-server", "version": "0.1.0"},
		})
	case "notifications/initialized":
		return nil
	case "tools/list":
		return makeResponse(reqID, map[string]interface{}{"tools": tools})
	case "tools/call":
		name, _ := params["name"].(string)
		arguments, _ := params["arguments"].(map[string]interface{})
		handler := toolHandlers[name]
		if handler == nil {
			return makeError(reqID, -32601, fmt.Sprintf("Unknown tool: %s", name))
		}
		return makeResponse(reqID, handler(arguments))
	default:
		return makeError(reqID, -32601, fmt.Sprintf("Method not found: %s", method))
	}
}

func serve(stdin io.Reader, stdout io.Writer) {
	r := bufio.NewReader(stdin)
	w := bufio.NewWriter(stdout)
	logger.Println("MCP server ready on stdio")
	for {
		request, err := readMessage(r)
		if err != nil {
			if err == io.EOF {
				logger.Println("EOF reached; shutting down")
				break
			}
			logger.Printf("Bad JSON: %v", err)
			_ = writeMessage(w, makeError(nil, -32700, "Parse error"))
			_ = w.Flush()
			continue
		}
		if request == nil {
			logger.Println("EOF reached; shutting down")
			break
		}
		response := handleRequest(request)
		if response != nil {
			_ = writeMessage(w, response)
			_ = w.Flush()
		}
	}
}

// ---------- smoke test ----------

func smokeTest() {
	requests := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]interface{}{"protocolVersion": "2024-11-05", "capabilities": map[string]interface{}{}},
		},
		{"jsonrpc": "2.0", "id": nil, "method": "notifications/initialized"},
		{"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
		{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params":  map[string]interface{}{"name": "get_current_time", "arguments": map[string]interface{}{"timezone": "UTC"}},
		},
		{
			"jsonrpc": "2.0",
			"id":      4,
			"method":  "tools/call",
			"params":  map[string]interface{}{"name": "calculate", "arguments": map[string]interface{}{"expression": "(2 + 3) * 4"}},
		},
	}

	var stdin bytes.Buffer
	for _, req := range requests {
		body, _ := json.Marshal(req)
		stdin.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	}

	var stdout bytes.Buffer
	serve(&stdin, &stdout)
	fmt.Println("Smoke test output:")
	fmt.Println(stdout.String())
}

func main() {
	smoke := flag.Bool("smoke", false, "Run a self-contained smoke test instead of serving.")
	flag.Parse()
	if *smoke {
		smokeTest()
	} else {
		serve(os.Stdin, os.Stdout)
	}
}
