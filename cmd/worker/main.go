package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"temporal-poc/src/config"
	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/nodes/activities"
	"temporal-poc/src/register"
	"temporal-poc/src/services"
	workflows "temporal-poc/src/workflows"
	"time"

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
func DynamicActivity(deps *core.Deps) func(ctx context.Context, args converter.EncodedValues) (activities.ActivityResult, error) {
	return func(ctx context.Context, args converter.EncodedValues) (activities.ActivityResult, error) {
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
			return activities.ActivityResult{}, fmt.Errorf("activity not found in register: %s", activityName)
		}

		// Execute the activity function with deps (dereference pointer to pass value)
		logger.Info("Executing activity from register", "ActivityName", activityName)
		result, err := activityFn(ctx, activityCtx, *deps)
		if err != nil {
			// Return error to trigger retries
			return result, err
		}
		return result, nil
	}
}

func main() {
	// Create a logger that ignores all warnings but keeps info and error messages
	baseLogger := core.NewLoggerWithoutWarnings()
	filteredLogger := core.NewFilteredLogger(baseLogger)

	// Load Temporal client configuration from .env file
	clientOptions := config.LoadTemporalClientOptions()
	clientOptions.Identity = fmt.Sprintf("worker-%s", uuid.New().String())
	clientOptions.Logger = filteredLogger

	// Create Temporal client
	c, err := client.Dial(clientOptions)
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	// Note: Client will be closed explicitly during graceful shutdown

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

	// Initialize dependencies - all deps must be initialized here
	leadService := services.NewLeadService()
	deps := core.NewDeps(leadService)

	// Create worker with stop timeout to ensure graceful shutdown
	w := worker.New(c, domain.PrimaryWorkflowTaskQueue, worker.Options{
		WorkerStopTimeout: 5 * time.Second, // Time to wait for tasks to finish before stopping
	})

	w.RegisterDynamicWorkflow(workflows.DynamicWorkflow, workflow.DynamicRegisterOptions{})

	// Register dynamic activity as fallback for any unregistered activities
	// Pass deps to DynamicActivity
	w.RegisterDynamicActivity(DynamicActivity(deps), activity.DynamicRegisterOptions{})

	// Register all nodes from register so they appear in the Temporal UI
	// Activities are registered with their actual functions from the register
	// Workflow tasks are registered with a placeholder (actual work is in workflow)
	nodeNames := register.GetAllNodeNames()
	log.Printf("Registering %d nodes from register", len(nodeNames))
	for _, nodeName := range nodeNames {
		log.Printf("Registering node: %s", nodeName)
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start worker in a goroutine
	workerErrChan := make(chan error, 1)
	go func() {
		log.Println("Worker started, listening on task queue: primary-workflow-task-queue")
		// Pass nil to Run() so we handle signals ourselves
		err := w.Run(worker.InterruptCh())
		workerErrChan <- err
	}()

	// Wait for interrupt signal or worker error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

		// Stop the worker - this stops accepting new tasks and stops polling
		w.Stop()
		log.Println("Worker stop requested, waiting for graceful shutdown...")

		// Create a context with timeout for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Wait for worker to finish processing and fully detach
		// Use a select to handle both the worker error channel and timeout
		select {
		case err := <-workerErrChan:
			if err != nil {
				log.Printf("Worker error during shutdown: %v", err)
			} else {
				log.Println("Worker stopped gracefully")
			}
		case <-shutdownCtx.Done():
			log.Println("Worker shutdown timeout reached, forcing stop")
		}

		// Explicitly close the client connection to detach from Temporal
		// This closes all connections and stops any remaining polling
		if c != nil {
			log.Println("Closing Temporal client connection...")
			c.Close()
			log.Println("Worker detached from Temporal")
		}

		// Small delay to ensure connection is fully closed
		time.Sleep(100 * time.Millisecond)
		log.Println("Shutdown complete")

	case err := <-workerErrChan:
		if err != nil {
			log.Fatalln("Unable to start worker", err)
		}
	}
}
