package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
)

type taskFunc func(deps map[string]interface{}) (interface{}, error)

type Task struct {
	Name    string
	Func    taskFunc
	Deps    []string
	Retries int
	Result  interface{}
	Error   error
}

type WorkflowEngine struct {
	tasks map[string]*Task
}

func newWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{tasks: map[string]*Task{}}
}

func (e *WorkflowEngine) addTask(t *Task) {
	e.tasks[t.Name] = t
}

func (e *WorkflowEngine) topologicalSort() ([]string, error) {
	inDegree := map[string]int{}
	dependents := map[string][]string{}
	for name := range e.tasks {
		inDegree[name] = 0
		dependents[name] = nil
	}
	for name, t := range e.tasks {
		for _, dep := range t.Deps {
			if _, ok := e.tasks[dep]; !ok {
				return nil, fmt.Errorf("task %s depends on unknown task %s", name, dep)
			}
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var ordered []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ordered = append(ordered, current)
		for _, dependent := range dependents[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(ordered) != len(e.tasks) {
		return nil, fmt.Errorf("cycle detected in task dependencies")
	}
	return ordered, nil
}

func (e *WorkflowEngine) run() (map[string]interface{}, error) {
	order, err := e.topologicalSort()
	if err != nil {
		return nil, err
	}
	log.Printf("Execution order: %v", order)
	for _, name := range order {
		t := e.tasks[name]
		depsResults := map[string]interface{}{}
		for _, dep := range t.Deps {
			depsResults[dep] = e.tasks[dep].Result
		}
		for attempt := 1; attempt <= t.Retries; attempt++ {
			log.Printf("Running task '%s' (attempt %d/%d)", name, attempt, t.Retries)
			result, err := t.Func(depsResults)
			if err == nil {
				t.Result = result
				t.Error = nil
				break
			}
			t.Error = err
			log.Printf("Task '%s' attempt %d failed: %v", name, attempt, err)
			if attempt == t.Retries {
				log.Printf("Task '%s' exhausted retries", name)
				return nil, err
			}
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		log.Printf("Task '%s' completed", name)
	}

	results := map[string]interface{}{}
	for name, t := range e.tasks {
		results[name] = t.Result
	}
	return results, nil
}

func fetchData(deps map[string]interface{}) (interface{}, error) {
	return map[string]string{
		"title":   "AI Agent Engineering",
		"content": "Workflow orchestration is essential for reliable agent systems.",
	}, nil
}

func makeAnalyzeSentiment(client *openaiclient.Client) taskFunc {
	return func(deps map[string]interface{}) (interface{}, error) {
		fetch := deps["fetch"].(map[string]string)
		text := fetch["content"]
		if client == nil {
			log.Println("No LLM available; using deterministic sentiment fallback")
			return "positive", nil
		}
		resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages: []openaiclient.Message{
				{Role: "system", Content: "Classify sentiment as exactly one word: positive, negative, or neutral."},
				{Role: "user", Content: text},
			},
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(10),
		})
		if err != nil {
			return "", err
		}
		return strings.ToLower(strings.TrimSpace(resp.Choices[0].Message.Content)), nil
	}
}

func makeFlakyQualityCheck() taskFunc {
	callCount := 0
	return func(deps map[string]interface{}) (interface{}, error) {
		callCount++
		if callCount < 3 {
			return "", fmt.Errorf("quality check service unavailable (attempt %d)", callCount)
		}
		sentiment := deps["sentiment"].(string)
		return fmt.Sprintf("quality_ok (%s)", sentiment), nil
	}
}

func makeSummarize(client *openaiclient.Client) taskFunc {
	return func(deps map[string]interface{}) (interface{}, error) {
		fetch := deps["fetch"].(map[string]string)
		sentiment := deps["sentiment"].(string)
		if client == nil {
			log.Println("No LLM available; using deterministic summary fallback")
			return fmt.Sprintf("Summary: '%s' has %s sentiment.", fetch["title"], sentiment), nil
		}
		prompt := fmt.Sprintf("Title: %s\nContent: %s\nSentiment: %s\nWrite a one-sentence summary.", fetch["title"], fetch["content"], sentiment)
		resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages:    []openaiclient.Message{{Role: "user", Content: prompt}},
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(60),
		})
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(resp.Choices[0].Message.Content), nil
	}
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

	engine := newWorkflowEngine()
	engine.addTask(&Task{Name: "fetch", Func: fetchData, Retries: 2})
	engine.addTask(&Task{Name: "sentiment", Func: makeAnalyzeSentiment(client), Deps: []string{"fetch"}, Retries: 2})
	engine.addTask(&Task{Name: "quality", Func: makeFlakyQualityCheck(), Deps: []string{"sentiment"}, Retries: 5})
	engine.addTask(&Task{Name: "summary", Func: makeSummarize(client), Deps: []string{"fetch", "sentiment"}, Retries: 2})

	fmt.Println("Starting DAG workflow...")
	results, err := engine.run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Workflow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nFinal results:")
	b, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(b))
}
