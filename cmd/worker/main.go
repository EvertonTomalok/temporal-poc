package main

import (
	"fmt"
	"log"
	"temporal-poc/src/core"
	"temporal-poc/src/register"
	workflows "temporal-poc/src/workflows"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create a logger that ignores all warnings but keeps info and error messages
	baseLogger := core.NewLoggerWithoutWarnings()
	filteredLogger := core.NewFilteredLogger(baseLogger)

	// Create Temporal client with filtered logger
	c, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
		Identity: fmt.Sprintf("worker-%s", uuid.New().String()),
		Logger:   filteredLogger,
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Register search attributes if they don't exist
	// This MUST succeed before workflows can run, otherwise workflows will fail with BadSearchAttributes
	log.Println("Checking and registering search attributes...")
	if err := core.RegisterSearchAttributesIfNeeded(c); err != nil {
		log.Printf("ERROR: Failed to register search attributes automatically: %v", err)
		log.Println("")
		log.Println("Workflows will fail with BadSearchAttributes error if search attributes are not registered.")
		log.Println("Please register them manually using one of these methods:")
		log.Println("")
		log.Println("  Method 1 (Temporal CLI):")
		log.Println("    temporal operator search-attributes add -name ClientAnswered -type Bool")
		log.Println("    temporal operator search-attributes add -name ClientAnsweredAt -type Datetime")
		log.Println("")
		log.Println("  Method 2 (Temporal Server startup):")
		log.Println("    temporal server start-dev \\")
		log.Println("      --search-attribute ClientAnswered=Bool \\")
		log.Println("      --search-attribute ClientAnsweredAt=Datetime")
		log.Println("")
		log.Fatalln("Worker cannot start without registered search attributes. Exiting.")
	}
	log.Println("✓ Search attributes verified/registered successfully")

	// Create worker
	w := worker.New(c, core.PrimaryWorkflowTaskQueue, worker.Options{})

	// Register workflow
	w.RegisterWorkflow(workflows.Workflow)

	// Register all named activities so they appear with node names in the Temporal UI
	// Each activity is registered with its node name so it appears correctly in the UI
	// We use the same ProcessNodeActivity function for all nodes, but register it with different names
	nodeNames := register.GetAllRegisteredNodeNames()
	log.Printf("Registering %d named activities for UI display", len(nodeNames))
	for _, nodeName := range nodeNames {
		// Register the same activity function with different names for UI display
		w.RegisterActivityWithOptions(register.ProcessNodeActivity, activity.RegisterOptions{
			Name: nodeName,
		})
		log.Printf("  Registered activity: %s", nodeName)
	}

	// Note: ProcessNodeActivity is no longer registered to force use of named activities
	// This ensures the correct node name appears in the Temporal UI

	// Start worker
	log.Println("Worker started, listening on task queue: primary-workflow-task-queue")
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
