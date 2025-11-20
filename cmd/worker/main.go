package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"temporal-poc/src/core"
	workflows "temporal-poc/src/workflows"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	tlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create a filtered logger to suppress HTTP 204 warnings
	// Use slog with a simple text handler as the base logger
	baseLogger := tlog.NewStructuredLogger(
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	)
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

	// Create worker
	w := worker.New(c, core.PrimaryWorkflowTaskQueue, worker.Options{})

	// Register workflow
	w.RegisterWorkflow(workflows.SignalCollectorWorkflow)

	// Start worker
	log.Println("Worker started, listening on task queue: primary-workflow-task-queue")
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
