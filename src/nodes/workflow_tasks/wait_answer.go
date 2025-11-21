package nodes

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
)

var WaitAnswerName = "wait_answer"

func init() {
	// Register node with container (processor and workflow node)
	// No retry policy - pass nil for empty retry policy
	// This is a workflow task because it waits for signals and uses timers
	RegisterNode(WaitAnswerName, waitAnswerProcessorNode, nil, NodeTypeWorkflowTask)
}

// WaitAnswerWorkflowNode is the workflow node that handles waiting for client-answered signal or timeout
// It returns whether to continue to the next node or stop the flow
// This node will wait in a loop for a maximum of 60 seconds if no signal is received
func waitAnswerProcessorNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("WaitAnswerWorkflowNode: Starting wait_answer - will wait up to 60 seconds for client-answered signal")

	// This node always uses its own fixed timeout (1 minute / 60 seconds)
	// The workflow-level timeoutDuration is for the entire workflow, not individual nodes
	waitAnswerTimeout := 1 * time.Minute
	logger.Info("WaitAnswerWorkflowNode: Waiting for client-answered signal or timeout", "timeout_seconds", int(waitAnswerTimeout.Seconds()))

	// Create channel for client-answered signal
	clientAnsweredChannel := workflow.GetSignalChannel(ctx, domain.ClientAnsweredSignal)

	clientAnswered := false

	// Create timer outside the loop
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(timerCtx, waitAnswerTimeout)

	// Create selector outside the loop
	selector := workflow.NewSelector(ctx)

	// Wait for client-answered signal
	selector.AddReceive(clientAnsweredChannel, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil) // Receive the signal
		logger.Info("WaitAnswerWorkflowNode: client-answered signal received")
		clientAnswered = true
		cancelTimer()
	})

	// Add timer to selector
	selector.AddFuture(timer, func(f workflow.Future) {
		cancelTimer()
		logger.Info("WaitAnswerWorkflowNode: Timeout timer fired")
	})

	// Wait for either signal or timeout
	selector.Select(ctx)

	// Check if signal was received
	if clientAnswered {
		cancelTimer()
		logger.Info("WaitAnswerWorkflowNode: Client answered, processing")

		// Update search attributes from workflow context
		err := workflow.UpsertTypedSearchAttributes(
			ctx,
			core.ClientAnsweredField.ValueSet(true),
			core.ClientAnsweredAtField.ValueSet(workflow.Now(ctx).UTC()),
		)
		if err != nil {
			logger.Error("WaitAnswerWorkflowNode: Failed to upsert search attributes", "error", err)
		} else {
			logger.Info("WaitAnswerWorkflowNode: Successfully upserted client-answered search attributes")
		}

		logger.Info("WaitAnswerWorkflowNode: Processing completed")
		// Return result with activity information - executor will call ExecuteActivity
		// Continue to next node (notify_creator) when signal is received
		return NodeExecutionResult{
			Error:        nil,
			ActivityName: WaitAnswerName,
			EventType:    domain.EventTypeConditionSatisfied,
		}
	}

	// Timer fired (timeout reached)
	logger.Info("WaitAnswerWorkflowNode: Timeout reached")
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: WaitAnswerName,
		EventType:    domain.EventTypeConditionTimeout,
	}
}
