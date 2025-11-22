package activities

import (
	"context"
	"fmt"
	"temporal-poc/src/core/domain"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

const NotifyCreatorActivityName = "notify_creator"

func init() {
	// Register with retry policy for automatic retries on failure
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2,
		MaximumInterval:    45 * time.Second,
		MaximumAttempts:    20,
	}
	// No schema defined for notify_creator (no input required)
	RegisterActivity(NotifyCreatorActivityName, NotifyCreatorActivity, retryPolicy, nil)
}

// NotifyCreatorActivity sends a notification to the creator
// This is a real Temporal activity that will be retried on failure
// First attempt fails to demonstrate retry behavior
func NotifyCreatorActivity(ctx context.Context, activityCtx ActivityContext) (ActivityResult, error) {
	logger := activity.GetLogger(ctx)

	// Get attempt number from activity info
	info := activity.GetInfo(ctx)
	attempt := int(info.Attempt)
	// Fail on attempts up to MaxAttemptsToFail to demonstrate retry behavior
	// Return error to trigger retries
	if attempt <= 3 {
		logger.Error("NOTIFY CREATOR: attempt failed (simulated failure)", "attempt", attempt)
		return ActivityResult{}, temporal.NewApplicationError(
			fmt.Sprintf("simulated failure on attempt %d", attempt),
			"RetryableError",
		)
	}

	logger.Info("NotifyCreatorActivity executing", "workflow_id", activityCtx.WorkflowID, "attempt", attempt)

	// Simulate notifying the creator
	// In a real implementation, this would make an HTTP call, send an email, etc.
	logger.Info("NOTIFY CREATOR: Sending notification to creator", "attempt", attempt)
	logger.Info("NOTIFY CREATOR PAYLOAD", "event", "client_answered", "workflow_id", activityCtx.WorkflowID)

	// Simulate potential failure - in real scenario, this could be a network call
	// If this fails, Temporal will retry according to the retry policy
	logger.Info("NOTIFY CREATOR RESPONSE: 200 OK")
	logger.Info("NotifyCreatorActivity completed successfully", "attempt", attempt)

	return ActivityResult{
		EventType: domain.EventTypeConditionSatisfied,
	}, nil
}
