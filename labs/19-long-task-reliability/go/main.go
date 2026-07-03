package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

var checkpointDir = ".checkpoints"

type checkpointStore struct {
	path string
}

func newCheckpointStore(key string) *checkpointStore {
	return &checkpointStore{path: filepath.Join(checkpointDir, key+".json")}
}

func (s *checkpointStore) load() map[string]interface{} {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return map[string]interface{}{}
	}
	var state map[string]interface{}
	_ = json.Unmarshal(b, &state)
	return state
}

func (s *checkpointStore) save(state map[string]interface{}) error {
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func exponentialBackoff(attempt int, baseDelay, maxDelay float64) time.Duration {
	delay := math.Min(baseDelay*math.Pow(2, float64(attempt-1)), maxDelay)
	jitter := delay * 0.1
	if attempt%2 == 0 {
		jitter = -jitter
	}
	return time.Duration((delay+jitter)*1000) * time.Millisecond
}

func runWithTimeout(fn func() (interface{}, error), timeout time.Duration) (interface{}, error) {
	type result struct {
		val interface{}
		err error
	}
	ch := make(chan result, 1)
	go func() {
		val, err := fn()
		ch <- result{val, err}
	}()
	select {
	case r := <-ch:
		return r.val, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("step timed out after %v", timeout)
	}
}

type longTask struct {
	inputData       string
	client          *openaiclient.Client
	idempotencyKey  string
	store           *checkpointStore
	state           map[string]interface{}
	flakyAttempts   int
}

func newLongTask(inputData string, client *openaiclient.Client) *longTask {
	key := makeKey(inputData)
	store := newCheckpointStore(key)
	state := store.load()
	return &longTask{
		inputData:      inputData,
		client:         client,
		idempotencyKey: key,
		store:          store,
		state:          state,
	}
}

func makeKey(inputData string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(inputData))
	return "task-" + hex.EncodeToString(h.Sum(nil))
}

func (t *longTask) isComplete() bool {
	status, _ := t.state["status"].(string)
	key, _ := t.state["idempotency_key"].(string)
	return status == "completed" && key == t.idempotencyKey
}

func (t *longTask) runStep(name string, fn func() (interface{}, error), timeout time.Duration, maxRetries int) (interface{}, error) {
	completedSteps, _ := t.state["completed_steps"].(map[string]interface{})
	if completedSteps == nil {
		completedSteps = map[string]interface{}{}
	}
	if completedSteps[name] == true {
		log.Printf("Step '%s' already completed; skipping", name)
		results, _ := t.state["results"].(map[string]interface{})
		return results[name], nil
	}

	log.Printf("Executing step '%s'", name)
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := runWithTimeout(fn, timeout)
		if err == nil {
			if completedSteps == nil {
				completedSteps = map[string]interface{}{}
			}
			completedSteps[name] = true
			t.state["completed_steps"] = completedSteps
			results, _ := t.state["results"].(map[string]interface{})
			if results == nil {
				results = map[string]interface{}{}
			}
			results[name] = result
			t.state["results"] = results
			t.state["last_step"] = name
			if err := t.store.save(t.state); err != nil {
				log.Printf("Checkpoint save failed: %v", err)
			}
			log.Printf("Step '%s' succeeded", name)
			return result, nil
		}
		lastErr = err
		log.Printf("Step '%s' attempt %d failed: %v", name, attempt, err)
		if attempt < maxRetries {
			delay := exponentialBackoff(attempt, 1.0, 30.0)
			log.Printf("Retrying step '%s' in %.2fs", name, delay.Seconds())
			time.Sleep(delay)
		}
	}
	log.Printf("Step '%s' exhausted retries", name)
	return nil, lastErr
}

func (t *longTask) stepFetchData() (interface{}, error) {
	log.Printf("Fetching data for: %s", t.inputData)
	if t.client != nil {
		resp, err := t.client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages:  []openaiclient.Message{{Role: "user", Content: fmt.Sprintf("Summarize '%s' in one sentence.", t.inputData)}},
			MaxTokens: intPtr(50),
		})
		if err != nil {
			return nil, err
		}
		return strings.TrimSpace(resp.Choices[0].Message.Content), nil
	}
	return fmt.Sprintf("Mock summary for '%s'.", t.inputData), nil
}

func (t *longTask) stepProcessData(fetched string) (interface{}, error) {
	t.flakyAttempts++
	if t.flakyAttempts < 3 {
		return nil, fmt.Errorf("processing service busy (attempt %d)", t.flakyAttempts)
	}
	return fmt.Sprintf("processed(%s)", fetched), nil
}

func (t *longTask) stepNotify(processed string) (interface{}, error) {
	return fmt.Sprintf("notification_sent(%s)", processed), nil
}

func (t *longTask) run() (map[string]interface{}, error) {
	if t.isComplete() {
		log.Printf("Task already completed for key %s; returning cached result", t.idempotencyKey)
		results, _ := t.state["results"].(map[string]interface{})
		return map[string]interface{}{"status": "completed", "results": results}, nil
	}

	t.state["idempotency_key"] = t.idempotencyKey
	t.state["status"] = "running"
	if _, ok := t.state["completed_steps"]; !ok {
		t.state["completed_steps"] = map[string]interface{}{}
	}
	if _, ok := t.state["results"]; !ok {
		t.state["results"] = map[string]interface{}{}
	}

	fetched, err := t.runStep("fetch", t.stepFetchData, 5*time.Second, 3)
	if err != nil {
		return nil, err
	}
	processed, err := t.runStep("process", func() (interface{}, error) { return t.stepProcessData(fetched.(string)) }, 5*time.Second, 3)
	if err != nil {
		return nil, err
	}
	notified, err := t.runStep("notify", func() (interface{}, error) { return t.stepNotify(processed.(string)) }, 5*time.Second, 3)
	if err != nil {
		return nil, err
	}
	_ = notified

	t.state["status"] = "completed"
	if err := t.store.save(t.state); err != nil {
		log.Printf("Final checkpoint save failed: %v", err)
	}
	log.Println("Task completed successfully")
	results, _ := t.state["results"].(map[string]interface{})
	return map[string]interface{}{"status": "completed", "results": results}, nil
}

func intPtr(i int) *int {
	return &i
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	cfg := openaiclient.LoadConfig()
	var client *openaiclient.Client
	if cfg.APIKey != "" || strings.HasPrefix(cfg.BaseURL, "http://localhost") {
		client = openaiclient.NewClient()
	} else {
		log.Println("LLM client disabled: OPENAI_API_KEY not set")
	}

	inputData := "reliable agent engineering"
	task := newLongTask(inputData, client)

	fmt.Println("Starting long-running task...")
	fmt.Println("Idempotency key:", task.idempotencyKey)
	result, err := task.run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Task failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nFinal result:")
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))

	fmt.Println("\nRe-running with the same idempotency key...")
	task2 := newLongTask(inputData, client)
	result2, err := task2.run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Task failed: %v\n", err)
		os.Exit(1)
	}
	b, _ = json.MarshalIndent(result2, "", "  ")
	fmt.Println(string(b))
}
