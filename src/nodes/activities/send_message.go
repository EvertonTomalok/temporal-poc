package activities

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

const SendMessageActivityName = "send_message"

func init() {
	// Register with retry policy for automatic retries on failure
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    15,
	}
	RegisterActivity(SendMessageActivityName, SendMessageActivity, retryPolicy)
}

// SendMessageActivity sends a message to the client
// This is a real Temporal activity that will be retried on failure
func SendMessageActivity(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("SendMessageActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Simulate sending a message
	logger.Info("MESSAGE SENT: Sending message to client")
	logger.Info("MESSAGE PAYLOAD", "workflow_id", activityCtx.WorkflowID, "event", "message_sent")

	info := activity.GetInfo(ctx)
	attempt := info.Attempt
	percentFailure := rand.Intn(100)
	if attempt <= 2 && percentFailure < 50 {
		logger.Error("SEND MESSAGE: attempt failed (simulated failure)", "attempt", attempt)
		return temporal.NewApplicationError(
			fmt.Sprintf("simulated failure on attempt %d", int(attempt)),
			"RetryableError",
		)
	}

	now := time.Now()
	deterministicValue := int(now.UnixNano() % 1500)
	sleepDuration := time.Duration(deterministicValue+500) * time.Millisecond

	logger.Info("Sleeping before response", "duration", sleepDuration)
	time.Sleep(sleepDuration)

	logger.Info("MESSAGE RESPONSE: 200 OK")
	logger.Info("SendMessageActivity completed successfully")

	return nil
}
