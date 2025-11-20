package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

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

	// Create worker
	w := worker.New(c, "signal-collector-task-queue", worker.Options{})

	// Register workflow
	w.RegisterWorkflow(workflows.SignalCollectorWorkflow)

	// Start worker
	log.Println("Worker started, listening on task queue: signal-collector-task-queue")
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
