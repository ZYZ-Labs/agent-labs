package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	openaiclient "github.com/ZYZ-Labs/agent-labs/shared/config"
	"go.temporal.io/sdk/client"
)

func main() {
	// The Python client only configures logging; reuse the shared helper.
	_ = openaiclient.LoadConfig()

	c, err := client.Dial(client.Options{
		HostPort:  "localhost:7233",
		Namespace: "default",
	})
	if err != nil {
		log.Printf("Could not connect to Temporal server at localhost:7233: %v", err)
		fmt.Println("\nStart Temporal first with:")
		fmt.Println("  docker compose up -d")
		fmt.Println("Then start the worker:")
		fmt.Println("  go run worker.go")
		os.Exit(1)
	}
	defer c.Close()

	workflowID := "greeting-workflow-001"
	fmt.Printf("Starting workflow %s...\n", workflowID)
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}
	we, err := c.ExecuteWorkflow(context.Background(), options, greetingWorkflow, "Agent Engineer")
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}
	fmt.Println("Workflow result:", result)
}
