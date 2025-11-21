package activities

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

const SendMessageActivityName = "send_message"

func init() {
	RegisterActivity(SendMessageActivityName, SendMessageActivity)
}

// SendMessageActivity sends a message to the client
// This is a real Temporal activity that will be retried on failure
func SendMessageActivity(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("SendMessageActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Simulate sending a message
	logger.Info("MESSAGE SENT: Sending message to client")
	logger.Info("MESSAGE PAYLOAD", "workflow_id", activityCtx.WorkflowID, "event", "message_sent")

	// Simulate network delay - in a real implementation, this would be an HTTP call
	// Use time.Sleep in activities (not workflow.Sleep) since we're in an activity context
	// Generate deterministic duration between 500ms (0.5s) and 2000ms (2s)
	// Using activity context info to create deterministic "random" value
	now := time.Now()
	deterministicValue := int(now.UnixNano() % 1500)
	sleepDuration := time.Duration(deterministicValue+500) * time.Millisecond

	logger.Info("Sleeping before response", "duration", sleepDuration)
	time.Sleep(sleepDuration)

	logger.Info("MESSAGE RESPONSE: 200 OK")
	logger.Info("SendMessageActivity completed successfully")

	return nil
}
