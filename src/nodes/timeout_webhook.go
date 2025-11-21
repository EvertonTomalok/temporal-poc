package nodes

import (
	"temporal-poc/src/core/domain"
	"time"

	"go.temporal.io/sdk/workflow"
)

var WebhookName = "webhook"

func init() {
	RegisterNode(WebhookName, processTimeoutWebhookNode)
}

// processTimeoutWebhookNode processes the timeout webhook node
// Note: The actual webhook work should ideally be done in an activity, but for now
// we do it here. The sleep must use workflow.Sleep to yield to the Temporal runtime.
func processTimeoutWebhookNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("WebhookWorkflowNode: Processing webhook event")

	// Now do the actual work delay
	// Use workflow.Sleep to yield to Temporal runtime - this is critical to avoid deadlock warnings
	// workflow.Sleep yields control back to Temporal, allowing the workflow to be properly managed
	workflow.Sleep(ctx, 2*time.Second)

	logger.Info("WebhookWorkflowNode: Processing completed")
	// Return result with activity information - executor will call ExecuteActivity
	// Workflow definition (GoTo/Condition) controls flow continuation, not this node
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: WebhookName,
		EventType:    domain.EventTypeConditionTimeout,
	}
}
