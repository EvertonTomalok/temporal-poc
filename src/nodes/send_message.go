package nodes

import (
	"context"
	"math/rand"
	"temporal-poc/src/core"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

var SendMessageName = "send_message"

func init() {
	// Register node with container (processor and workflow node)
	RegisterNode(SendMessageName, processSendMessageNode, SendMessageWorkflowNode)
}

// processSendMessageNode processes the send message node
func processSendMessageNode(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing send message node", "workflow_id", activityCtx.WorkflowID)

	// Simulate sending a message
	logger.Info("MESSAGE SENT: Sending message to client")
	logger.Info("MESSAGE PAYLOAD", "workflow_id", activityCtx.WorkflowID, "event", "message_sent")

	// Sleep for a random duration between 0.5 to 2 seconds
	// Generate random duration between 500ms (0.5s) and 2000ms (2s)
	randomMs := rand.Intn(1500) + 500
	sleepDuration := time.Duration(randomMs) * time.Millisecond

	logger.Info("Sleeping before response", "duration", sleepDuration)
	// Use select to respect context cancellation (Temporal activity cancellation)
	select {
	case <-time.After(sleepDuration):
		// Sleep completed normally
	case <-ctx.Done():
		// Activity was cancelled, return the cancellation error
		logger.Info("Activity cancelled during sleep")
		return ctx.Err()
	}

	logger.Info("MESSAGE RESPONSE: 200 OK")
	logger.Info("Send message node processed successfully")
	return nil
}

// SendMessageWorkflowNode is the workflow node that handles sending a message
// It orchestrates the message sending by executing the activity, then continues to the next node
func SendMessageWorkflowNode(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("SendMessageWorkflowNode: Orchestrating message sending")

	// The actual work (sending message and sleeping) is done in the activity
	// This workflow node just orchestrates and returns the result
	// The activity will be executed by ExecuteActivity after this returns

	logger.Info("SendMessageWorkflowNode: Ready to execute activity, continuing to next node")
	// Return result with activity information - executor will call ExecuteActivity
	// Continue to next node (wait_answer) after activity completes
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: SendMessageName,
		EventType:    core.EventTypeSatisfied,
	}
}
