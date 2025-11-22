package main

import (
	"context"
	"fmt"
	"log"
	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/nodes/activities"
	"temporal-poc/src/register"
	workflows "temporal-poc/src/workflows"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// DynamicActivity handles all activity executions dynamically by reading from the register
// It routes to the appropriate activity function based on the activity name
// Uses converter.EncodedValues to accept dynamic arguments from the workflow
func DynamicActivity(ctx context.Context, args converter.EncodedValues) (activities.ActivityResult, error) {
	info := activity.GetInfo(ctx)
	activityName := info.ActivityType.Name

	logger := activity.GetLogger(ctx)
	logger.Info("DynamicActivity executing", "ActivityName", activityName)

	// Decode ActivityContext from the encoded values
	var activityCtx activities.ActivityContext
	if err := args.Get(&activityCtx); err != nil {
		logger.Error("Failed to decode ActivityContext", "error", err)
		return activities.ActivityResult{}, fmt.Errorf("failed to decode ActivityContext: %w", err)
	}

	// Get the activity function from the register
	reg := register.GetInstance()
	activityFn, exists := reg.GetActivityFunction(activityName)
	if !exists {
		// If it's not an activity, it might be a workflow task - use placeholder
		logger.Info("Activity not found in register, using placeholder", "ActivityName", activityName)
		// This is a placeholder for UI display only.
		// The actual processor is called from the workflow node in workflows/workflow.go.
		return activities.ActivityResult{}, nil
	}

	// Execute the activity function
	logger.Info("Executing activity from register", "ActivityName", activityName)
	result, err := activityFn(ctx, activityCtx)
	if err != nil {
		// Return error to trigger retries
		return result, err
	}
	return result, nil
}

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
	w := worker.New(c, domain.PrimaryWorkflowTaskQueue, worker.Options{})

	w.RegisterDynamicWorkflow(workflows.DynamicWorkflow, workflow.DynamicRegisterOptions{})

	// Register dynamic activity as fallback for any unregistered activities
	w.RegisterDynamicActivity(DynamicActivity, activity.DynamicRegisterOptions{})

	// Register all nodes from register so they appear in the Temporal UI
	// Activities are registered with their actual functions from the register
	// Workflow tasks are registered with a placeholder (actual work is in workflow)
	nodeNames := register.GetAllNodeNames()
	log.Printf("Registering %d nodes from register", len(nodeNames))
	for _, nodeName := range nodeNames {
		log.Printf("Registering node: %s", nodeName)
	}

	// Start worker
	log.Println("Worker started, listening on task queue: primary-workflow-task-queue")
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
