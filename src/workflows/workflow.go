package src

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/nodes"
)

// AbandonedCartWorkflow is an agnostic workflow that delegates to nodes
// All business logic is handled by nodes in the /nodes directory
func AbandonedCartWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("AbandonedCartWorkflow started")

	workflowInfo := workflow.GetInfo(ctx)
	workflowID := workflowInfo.WorkflowExecution.ID

	// Initialize workflow state
	startTime := workflow.Now(ctx)
	timeoutDuration := 1 * time.Minute

	// Build activity registry with node execution order
	// Hardcoded for now: wait_answer first, then webhook
	// This will be dynamically configured in the future (e.g., from a drag-and-drop UI)
	registry := nodes.NewActivityRegistry("wait_answer", "webhook")

	// Execute workflow nodes - registry orchestrates the flow
	// Starts with the first node (wait_answer), then continues to next node if instructed
	if err := registry.Execute(ctx, workflowID, startTime, timeoutDuration); err != nil {
		logger.Error("AbandonedCartWorkflow: Error executing workflow nodes", "error", err)
		return err
	}

	logger.Info("AbandonedCartWorkflow finishing")
	return nil
}
