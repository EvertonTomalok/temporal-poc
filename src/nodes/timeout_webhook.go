package nodes

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

func init() {
	// Register node with container (processor and workflow node)
	RegisterNode("webhook", processTimeoutWebhookNode, WebhookWorkflowNode)
}

// processTimeoutWebhookNode processes the timeout webhook node
func processTimeoutWebhookNode(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing timeout webhook node", "workflow_id", activityCtx.WorkflowID)

	// Simulate webhook call
	logger.Info("WEBHOOK CALL: POST /webhook/timeout")
	logger.Info("WEBHOOK PAYLOAD", "event", "timeout", "workflow_id", activityCtx.WorkflowID)
	logger.Info("WEBHOOK RESPONSE: 200 OK")

	// Note: Memo updates must be done from workflow context, not activity context
	// So we'll need to return information that the workflow can use to update memo
	logger.Info("Timeout webhook node processed successfully")

	return nil
}

// WebhookWorkflowNode is the workflow node that handles webhook processing
// It returns whether to continue to the next node or stop the flow
func WebhookWorkflowNode(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("WebhookWorkflowNode: Processing webhook event")

	// Determine event type based on whether client answered
	// This is a simplified check - in a real scenario, you'd track this state
	eventType := "webhook"

	// Use workflow.Sleep instead of time.Sleep in workflow functions
	workflow.Sleep(ctx, 2*time.Second)

	logger.Info("WebhookWorkflowNode: Processing completed")
	// Return result with activity information - executor will call ExecuteActivity
	// Stop the flow after webhook processing
	return NodeExecutionResult{
		ShouldContinue: false,
		Error:          nil,
		ActivityName:   "webhook",
		ClientAnswered: false,
		EventType:      eventType,
	}
}

// TimeoutWebhookNode handles timeout events and processes them
// This function contains all the logic for handling timeout events
// DEPRECATED: Use WebhookWorkflowNode instead
func TimeoutWebhookNode(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("TimeoutWebhookNode: Processing timeout event")

	// Note: Activity execution should be handled by executor, not here
	// This function is deprecated - use WebhookWorkflowNode instead

	// Update memo to record timeout event
	memo := map[string]interface{}{
		"timeout_occurred": true,
		"timeout_at":       workflow.Now(ctx).UTC(),
		"event":            "timeout",
		"workflow_id":      workflowID,
	}
	err := workflow.UpsertMemo(ctx, memo)
	if err != nil {
		logger.Error("TimeoutWebhookNode: Failed to upsert memo", "error", err)
	} else {
		logger.Info("TimeoutWebhookNode: Successfully updated memo with timeout information")
	}

	logger.Info("TimeoutWebhookNode: Processing completed")
	return nil
}
