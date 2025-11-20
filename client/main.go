package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"temporal-poc/workflows"
)

func main() {
	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Start workflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        "signal-collector-" + uuid.New().String(),
		TaskQueue: "signal-collector-task-queue",
	}

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, workflows.SignalCollectorWorkflow)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	workflowID := we.GetID()
	log.Printf("Workflow ID: %s\n", workflowID)
	log.Printf("Started workflow with ID: %s and RunID: %s\n", workflowID, we.GetRunID())

	// Wait for client-answered signal with 1 minute timeout
	log.Println("Waiting for 'client-answered' signal (max 1 minute)...")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Wait for workflow to complete (which happens after receiving client-answered signal)
	var result []workflows.SignalMessage
	err = we.Get(ctx, &result)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("Timeout: Did not receive 'client-answered' signal within 1 minute")
		} else {
			log.Fatalln("Error waiting for workflow:", err)
		}
	} else {
		log.Println("Received 'client-answered' signal - workflow completed")
	}
}
