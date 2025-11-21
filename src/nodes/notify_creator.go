package nodes

import (
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core/domain"
)

var NotifyCreatorName = "notify_creator"

func init() {
	// Register node with container (processor and workflow node)
	// No retry policy - pass nil for empty retry policy
	RegisterNode(NotifyCreatorName, processNotifyCreatorNode, nil)
}

// processNotifyCreatorNode processes the notify creator node
// This node only runs when wait_answer stops by signal (ClientAnswered = true)
func processNotifyCreatorNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("Processing notify creator node", "workflow_id", activityCtx.WorkflowID)

	// Only process if client answered (signal received)
	// If this node is called but client didn't answer, it means it was called after timeout
	// In that case, we should skip the notification
	if !activityCtx.ClientAnswered || activityCtx.EventType != domain.EventTypeConditionSatisfied {
		logger.Info("Skipping notify creator - client did not answer (timeout occurred)")
		return NodeExecutionResult{
			Error:        nil,
			ActivityName: NotifyCreatorName,
			EventType:    domain.EventTypeConditionTimeout,
		}
	}

	// Simulate notifying the creator
	logger.Info("NOTIFY CREATOR: Sending notification to creator")
	logger.Info("NOTIFY CREATOR PAYLOAD", "event", "client_answered", "workflow_id", activityCtx.WorkflowID)
	logger.Info("NOTIFY CREATOR RESPONSE: 200 OK")
	logger.Info("Notify creator node processed successfully")

	return NodeExecutionResult{
		Error:        nil,
		ActivityName: NotifyCreatorName,
		EventType:    domain.EventTypeConditionSatisfied,
	}
}
