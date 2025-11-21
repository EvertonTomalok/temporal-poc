package activities

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

const TimeoutWebhookActivityName = "webhook"

func init() {
	// Register with retry policy for automatic retries on failure
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    15,
	}
	RegisterActivity(TimeoutWebhookActivityName, TimeoutWebhookActivity, retryPolicy)
}

// TimeoutWebhookActivity sends a webhook notification on timeout
// This is a real Temporal activity that will be retried on failure
func TimeoutWebhookActivity(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("TimeoutWebhookActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Simulate webhook processing delay
	// In a real implementation, this would make an HTTP call to a webhook endpoint
	logger.Info("WebhookWorkflowNode: Processing webhook event")

	// Use SleepWithHeartbeat to keep the activity alive during long operations
	// In a real scenario, this would be the time to make the HTTP call
	// Heartbeat every 500ms to keep the activity responsive
	SleepWithHeartbeat(ctx, 3*time.Second, 500*time.Millisecond)

	logger.Info("WebhookWorkflowNode: Processing completed")
	logger.Info("TimeoutWebhookActivity completed successfully")

	return nil
}
