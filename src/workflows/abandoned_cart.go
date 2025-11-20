package src

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/register"
)

var AbandonedCartWorkflowName = "abandoned_cart"

func GenerateAbandonedCartWorkflowID() string {
	return fmt.Sprintf("%s-%s", AbandonedCartWorkflowName, uuid.New().String())
}

// AbandonedCartWorkflow is an agnostic workflow that delegates to nodes
// All business logic is handled by nodes in the /nodes directory
func AbandonedCartWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("AbandonedCartWorkflow started")

	workflowInfo := workflow.GetInfo(ctx)
	workflowID := workflowInfo.WorkflowExecution.ID

	// Initialize workflow state
	startTime := workflow.Now(ctx)

	// Build activity registry with node execution order
	// Hardcoded for now: send_message first, then wait_answer, then notify_creator (if signal), then webhook (if timeout)
	// This will be dynamically configured in the future (e.g., from a drag-and-drop UI)
	registry := register.NewActivityRegistry("send_message", "wait_answer", "notify_creator", "webhook")

	// Execute workflow nodes - registry orchestrates the flow
	// Starts with the first node (wait_answer), then continues to next node if instructed
	// Timeout is now handled by individual nodes (e.g., wait_answer defines its own timeout)
	if err := registry.Execute(ctx, workflowID, startTime, 24*30*time.Hour); err != nil {
		logger.Error("AbandonedCartWorkflow: Error executing workflow nodes", "error", err)
		return err
	}

	logger.Info("AbandonedCartWorkflow finishing")
	return nil
}
