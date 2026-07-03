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
	"regexp"
	"strconv"
	"strings"
)

var logger = log.New(os.Stderr, "lsp-server: ", log.LstdFlags)

var documents = map[string]string{}

func updateDocument(uri, text string) {
	documents[uri] = text
}

func findDefinition(uri string, position map[string]interface{}) map[string]interface{} {
	text := documents[uri]
	lines := strings.Split(text, "\n")
	lineIdx := 0
	if v, ok := position["line"].(float64); ok {
		lineIdx = int(v)
	}
	charIdx := 0
	if v, ok := position["character"].(float64); ok {
		charIdx = int(v)
	}
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil
	}
	line := lines[lineIdx]
	if charIdx >= len(line) {
		return nil
	}
	re := regexp.MustCompile(`[A-Za-z0-9_]+`)
	match := re.FindString(line[charIdx:])
	if match == "" {
		return nil
	}
	pattern := regexp.MustCompile(fmt.Sprintf(`^func\s+%s\s*\(`, regexp.QuoteMeta(match)))
	for docURI, docText := range documents {
		for _, m := range pattern.FindAllStringIndex(docText, -1) {
			startLine := strings.Count(docText[:m[0]], "\n")
			return map[string]interface{}{
				"uri": docURI,
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": startLine, "character": 0},
					"end":   map[string]interface{}{"line": startLine, "character": len(fmt.Sprintf("func %s(", match))},
				},
			}
		}
	}
	return nil
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
	return map[string]interface{}{"jsonrpc": "2.0", "id": requestID, "result": result}
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
			"capabilities": map[string]interface{}{
				"textDocumentSync":   map[string]interface{}{"openClose": true, "change": 0},
				"definitionProvider": true,
			},
			"serverInfo": map[string]interface{}{"name": "agent-labs-lsp", "version": "0.1.0"},
		})
	case "initialized":
		return nil
	case "textDocument/didOpen":
		td, _ := params["textDocument"].(map[string]interface{})
		updateDocument(td["uri"].(string), td["text"].(string))
		return nil
	case "textDocument/definition":
		td, _ := params["textDocument"].(map[string]interface{})
		pos, _ := params["position"].(map[string]interface{})
		return makeResponse(reqID, findDefinition(td["uri"].(string), pos))
	case "shutdown":
		return makeResponse(reqID, nil)
	case "exit":
		return nil
	default:
		return makeError(reqID, -32601, fmt.Sprintf("Method not found: %s", method))
	}
}

func serve(stdin io.Reader, stdout io.Writer) {
	r := bufio.NewReader(stdin)
	w := bufio.NewWriter(stdout)
	logger.Println("LSP server ready on stdio")
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
		method, _ := request["method"].(string)
		response := handleRequest(request)
		if response != nil {
			_ = writeMessage(w, response)
			_ = w.Flush()
		}
		if method == "exit" {
			break
		}
	}
}

// ---------- smoke test ----------

func smokeTest() {
	sampleCode := strings.Join([]string{
		`def greet(name):`,
		`    return f"Hello, {name}!"`,
		"",
		`print(greet("world"))`,
	}, "\n")
	uri := "file:///tmp/sample.py"

	requests := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]interface{}{"processId": nil, "rootUri": nil, "capabilities": map[string]interface{}{}},
		},
		{"jsonrpc": "2.0", "method": "initialized", "params": map[string]interface{}{}},
		{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri":        uri,
					"languageId": "python",
					"text":       sampleCode,
				},
			},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "textDocument/definition",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": uri},
				"position":     map[string]interface{}{"line": 3, "character": 6},
			},
		},
		{"jsonrpc": "2.0", "id": 3, "method": "shutdown"},
		{"jsonrpc": "2.0", "method": "exit"},
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
