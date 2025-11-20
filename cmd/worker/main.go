package main

import (
	"fmt"
	"log"
	workflows "temporal-poc/src/workflows"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
		Identity: fmt.Sprintf("worker-%s", uuid.New().String()),
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Create worker
	w := worker.New(c, "primary-workflow-task-queue", worker.Options{})

	// Register workflow
	w.RegisterWorkflow(workflows.SignalCollectorWorkflow)

	// Start worker
	log.Println("Worker started, listening on task queue: primary-workflow-task-queue")
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
