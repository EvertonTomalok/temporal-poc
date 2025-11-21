package nodes

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core"
)

var NotifyCreatorName = "notify_creator"

func init() {
	// Register node with container (processor and workflow node)
	RegisterNode(NotifyCreatorName, processNotifyCreatorNode, NotifyCreatorWorkflowNode)
}

// processNotifyCreatorNode processes the notify creator node
// This node only runs when wait_answer stops by signal (ClientAnswered = true)
func processNotifyCreatorNode(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing notify creator node", "workflow_id", activityCtx.WorkflowID)

	// Only process if client answered (signal received)
	// If this node is called but client didn't answer, it means it was called after timeout
	// In that case, we should skip the notification
	if !activityCtx.ClientAnswered || activityCtx.EventType != core.EventTypeSatisfied {
		logger.Info("Skipping notify creator - client did not answer (timeout occurred)")
		return nil
	}

	// Simulate notifying the creator
	logger.Info("NOTIFY CREATOR: Sending notification to creator")
	logger.Info("NOTIFY CREATOR PAYLOAD", "event", "client_answered", "workflow_id", activityCtx.WorkflowID)
	logger.Info("NOTIFY CREATOR RESPONSE: 200 OK")
	logger.Info("Notify creator node processed successfully")

	return nil
}

// NotifyCreatorWorkflowNode is the workflow node that handles notifying the creator
// It only runs when wait_answer stops by signal (not timeout)
func NotifyCreatorWorkflowNode(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("NotifyCreatorWorkflowNode: Orchestrating creator notification")

	sas := workflow.GetTypedSearchAttributes(ctx)
	clientAnswered, ok := sas.GetBool(core.ClientAnsweredField)

	if !ok || !clientAnswered {
		logger.Info("NotifyCreatorWorkflowNode: Client did not answer (timeout occurred), skipping notification")
		return NodeExecutionResult{
			Error:        nil,
			ActivityName: "notify_creator",
			EventType:    core.EventTypeTimeout,
		}
	}

	logger.Info("NotifyCreatorWorkflowNode: Client answered, will notify creator")
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: NotifyCreatorName,
		EventType:    core.EventTypeSatisfied,
	}
}
