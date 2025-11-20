package main

import (
	"context"
	"log"
	"temporal-poc/src/core"
	workflows "temporal-poc/src/workflows"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
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
		ID:        "abandonedCart-" + uuid.New().String(),
		TaskQueue: core.PrimaryWorkflowTaskQueue,
	}

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, workflows.AbandonedCartWorkflow)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	workflowID := we.GetID()
	log.Printf("Workflow ID: %s\n", workflowID)
	log.Printf("Started workflow with ID: %s and RunID: %s\n", workflowID, we.GetRunID())
	log.Printf("\n-> go run ./cmd/signal-sender -workflow-id \n %s to send 'client-answered' signal\n", workflowID)

	// Wait for client-answered signal with 1 minute timeout
	log.Println("Waiting for 'client-answered' signal (max 1 minute)...")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Wait for workflow to complete (which happens after receiving client-answered signal)
	err = we.Get(ctx, nil)
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
