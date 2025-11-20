package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

func main() {
	// Parse command-line arguments
	workflowID := flag.String("workflow-id", "", "Workflow ID to send signal to (required)")
	flag.Parse()

	if *workflowID == "" {
		flag.Usage()
		log.Fatalln("Error: workflow-id is required")
		os.Exit(1)
	}

	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
		Identity: fmt.Sprintf("signal-sender-%s", uuid.New().String()),
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Send client-answered signal to the workflow
	// Use empty RunID to signal the latest run
	err = c.SignalWorkflow(context.Background(), *workflowID, "", "client-answered", nil)
	if err != nil {
		log.Fatalln("Unable to signal workflow", err)
	}

	log.Printf("Successfully sent 'client-answered' signal to workflow: %s\n", *workflowID)
}
