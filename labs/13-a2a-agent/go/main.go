package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var logger = log.New(os.Stderr, "a2a-agent: ", log.LstdFlags)

var agentCard = map[string]interface{}{
	"name":        "agent-labs-echo-agent",
	"description": "A minimal A2A agent that echoes input after a short delay.",
	"url":         "http://localhost:8123",
	"version":     "0.1.0",
	"capabilities": map[string]interface{}{
		"streaming":         false,
		"pushNotifications": false,
	},
	"skills": []map[string]interface{}{
		{"id": "echo", "name": "Echo", "description": "Returns the input text as the task result."},
	},
}

var (
	tasks = map[string]map[string]interface{}{}
	taskLock sync.RWMutex
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b)
}

func createTask(message map[string]interface{}) map[string]interface{} {
	id := newUUID()
	task := map[string]interface{}{
		"id":        id,
		"status":    map[string]interface{}{"state": "submitted", "timestamp": time.Now().Unix()},
		"messages":  []map[string]interface{}{message},
		"artifacts": []interface{}{},
	}
	taskLock.Lock()
	tasks[id] = task
	taskLock.Unlock()

	go func() {
		time.Sleep(2 * time.Second)
		taskLock.Lock()
		task["status"] = map[string]interface{}{"state": "working", "timestamp": time.Now().Unix()}
		taskLock.Unlock()

		time.Sleep(2 * time.Second)
		parts, _ := message["parts"].([]interface{})
		text := ""
		if len(parts) > 0 {
			if p, ok := parts[0].(map[string]interface{}); ok {
				text, _ = p["text"].(string)
			}
		}
		taskLock.Lock()
		task["status"] = map[string]interface{}{
			"state":     "completed",
			"timestamp": time.Now().Unix(),
			"message": map[string]interface{}{
				"role":  "agent",
				"parts": []map[string]interface{}{{"type": "text", "text": fmt.Sprintf("Echo: %s", text)}},
			},
		}
		taskLock.Unlock()
	}()

	return task
}

func sendJSON(w http.ResponseWriter, status int, payload interface{}) {
	body, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func agentHandler(w http.ResponseWriter, r *http.Request) {
	logger.Printf("%s %s", r.Method, r.URL.Path)
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/.well-known/agent.json" {
			sendJSON(w, http.StatusOK, agentCard)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/tasks/") {
			id := r.URL.Path[len("/tasks/"):]
			taskLock.RLock()
			task, ok := tasks[id]
			taskLock.RUnlock()
			if ok {
				sendJSON(w, http.StatusOK, task)
			} else {
				sendJSON(w, http.StatusNotFound, map[string]string{"error": "Task not found"})
			}
			return
		}
		sendJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
	case http.MethodPost:
		if r.URL.Path == "/tasks/send" {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			message, _ := payload["message"].(map[string]interface{})
			task := createTask(message)
			sendJSON(w, http.StatusOK, task)
			return
		}
		sendJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
	default:
		sendJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
	}
}

func startServer(host string, port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", agentHandler)
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", host, port), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("Server error: %v", err)
		}
	}()
	logger.Printf("Agent server listening on http://%s:%d", host, port)
	return server
}

func fetchAgentCard(baseURL string) (map[string]interface{}, error) {
	resp, err := http.Get(baseURL + "/.well-known/agent.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var card map[string]interface{}
	if err := json.Unmarshal(body, &card); err != nil {
		return nil, err
	}
	return card, nil
}

func submitTask(baseURL, text string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"role":  "user",
			"parts": []map[string]interface{}{{"type": "text", "text": text}},
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/tasks/send", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var task map[string]interface{}
	if err := json.Unmarshal(respBody, &task); err != nil {
		return nil, err
	}
	return task, nil
}

func getTask(baseURL, id string) (map[string]interface{}, error) {
	resp, err := http.Get(fmt.Sprintf("%s/tasks/%s", baseURL, id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var task map[string]interface{}
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, err
	}
	return task, nil
}

func pollTask(baseURL, id string, timeout, interval time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := getTask(baseURL, id)
		if err != nil {
			return nil, err
		}
		status, _ := task["status"].(map[string]interface{})
		state, _ := status["state"].(string)
		logger.Printf("Task %s state: %s", id[:8], state)
		if state == "completed" || state == "failed" {
			return task, nil
		}
		time.Sleep(interval)
	}
	return nil, fmt.Errorf("task %s did not complete within %v", id, timeout)
}

func main() {
	baseURL := strings.TrimRight(os.Getenv("A2A_AGENT_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8123"
	}
	var ownServer *http.Server

	tryExisting := func() bool {
		_, err := fetchAgentCard(baseURL)
		return err == nil
	}

	if !tryExisting() {
		logger.Printf("No agent found at %s; starting local agent", baseURL)
		ownServer = startServer("localhost", 8123)
		for i := 0; i < 20; i++ {
			if tryExisting() {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	} else {
		logger.Printf("Using existing agent at %s", baseURL)
	}

	if ownServer != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = ownServer.Shutdown(ctx)
		}()
	}

	card, err := fetchAgentCard(baseURL)
	if err != nil {
		logger.Fatalf("Could not fetch agent card: %v", err)
	}
	fmt.Println("\n[Agent Card]")
	b, _ := json.MarshalIndent(card, "", "  ")
	fmt.Println(string(b))

	task, err := submitTask(baseURL, "Hello from the A2A client!")
	if err != nil {
		logger.Fatalf("Could not submit task: %v", err)
	}
	fmt.Println("\n[Submitted Task]")
	b, _ = json.MarshalIndent(task, "", "  ")
	fmt.Println(string(b))

	id, _ := task["id"].(string)
	final, err := pollTask(baseURL, id, 30*time.Second, 500*time.Millisecond)
	if err != nil {
		logger.Fatalf("Could not poll task: %v", err)
	}
	fmt.Println("\n[Final Task]")
	b, _ = json.MarshalIndent(final, "", "  ")
	fmt.Println(string(b))
}
