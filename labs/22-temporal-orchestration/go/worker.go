package main

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const taskQueue = "agent-labs-greeting-queue"

func composeGreeting(ctx context.Context, name string) (string, error) {
	activity.GetLogger(ctx).Info("Activity called", "name", name)
	return fmt.Sprintf("Hello, %s from Temporal!", name), nil
}

func greetingWorkflow(ctx workflow.Context, name string) (string, error) {
	workflow.GetLogger(ctx).Info("Workflow started", "name", name)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var result string
	err := workflow.ExecuteActivity(ctx, composeGreeting, name).Get(ctx, &result)
	return result, err
}

func runWorker() error {
	c, err := client.Dial(client.Options{HostPort: "localhost:7233"})
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(greetingWorkflow)
	w.RegisterActivity(composeGreeting)

	fmt.Println("Starting worker on task queue 'agent-labs-greeting-queue'...")
	return w.Run(worker.InterruptCh())
}
