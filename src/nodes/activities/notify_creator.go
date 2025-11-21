package activities

import (
	"context"
	"temporal-poc/src/core/domain"

	"go.temporal.io/sdk/activity"
)

const NotifyCreatorActivityName = "notify_creator"

func init() {
	RegisterActivity(NotifyCreatorActivityName, NotifyCreatorActivity)
}

// NotifyCreatorActivity sends a notification to the creator
// This is a real Temporal activity that will be retried on failure
func NotifyCreatorActivity(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("NotifyCreatorActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Only process if client answered (signal received)
	// If this activity is called but client didn't answer, it means it was called after timeout
	// In that case, we should skip the notification
	if !activityCtx.ClientAnswered || activityCtx.EventType != domain.EventTypeConditionSatisfied {
		logger.Info("Skipping notify creator - client did not answer (timeout occurred)")
		// Return nil to indicate success (we're skipping, not failing)
		return nil
	}

	// Simulate notifying the creator
	// In a real implementation, this would make an HTTP call, send an email, etc.
	logger.Info("NOTIFY CREATOR: Sending notification to creator")
	logger.Info("NOTIFY CREATOR PAYLOAD", "event", "client_answered", "workflow_id", activityCtx.WorkflowID)

	// Simulate potential failure - in real scenario, this could be a network call
	// If this fails, Temporal will retry according to the retry policy
	// For now, we'll just log success
	logger.Info("NOTIFY CREATOR RESPONSE: 200 OK")
	logger.Info("NotifyCreatorActivity completed successfully")

	return nil
}
