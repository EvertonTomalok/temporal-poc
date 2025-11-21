package nodes

import (
	"temporal-poc/src/core/domain"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var SendMessageName = "send_message"

func init() {
	// Register node with container (processor and workflow node)
	// Configure retry policy with exponential backoff for send_message node
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    15,
	}
	RegisterNode(SendMessageName, processSendMessageNode, retryPolicy)
}

// processSendMessageNode processes the send message node
func processSendMessageNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("Processing send message node", "workflow_id", activityCtx.WorkflowID)

	// Simulate sending a message
	logger.Info("MESSAGE SENT: Sending message to client")
	logger.Info("MESSAGE PAYLOAD", "workflow_id", activityCtx.WorkflowID, "event", "message_sent")

	// Sleep for a deterministic duration (using workflow.Now for deterministic "randomness")
	// Generate deterministic duration between 500ms (0.5s) and 2000ms (2s) based on workflow time
	// Using workflow.Now ensures determinism - the duration will be consistent across replays
	now := workflow.Now(ctx)
	// Use nanoseconds from workflow time to create deterministic "random" value
	deterministicValue := int(now.UnixNano() % 1500)
	sleepDuration := time.Duration(deterministicValue+500) * time.Millisecond

	logger.Info("Sleeping before response", "duration", sleepDuration)
	// Use workflow.Sleep instead of time.After for determinism - this yields to Temporal runtime
	workflow.Sleep(ctx, sleepDuration)
	// Sleep completed normally
	logger.Info("MESSAGE RESPONSE: 200 OK")
	logger.Info("Send message node processed successfully")
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: SendMessageName,
		EventType:    domain.EventTypeConditionSatisfied,
	}
}
