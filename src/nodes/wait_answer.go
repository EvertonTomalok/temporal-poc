package nodes

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core"
)

func init() {
	// Register node with container (processor and workflow node)
	RegisterNode("wait_answer", processClientAnsweredProcessorNode, WaitAnswerWorkflowNode)
}

// processClientAnsweredProcessorNode processes the client-answered processor node
func processClientAnsweredProcessorNode(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing wait_answer activity", "workflow_id", activityCtx.WorkflowID)
	logger.Info("wait_answer activity will wait up to 60 seconds for client-answered signal")

	// This activity processes the client-answered event
	// Note: The actual waiting happens in the workflow node, not in the activity
	// Search attributes must be updated from workflow context, not activity context
	logger.Info("Client-answered processor node processed successfully")

	return nil
}

// WaitAnswerWorkflowNode is the workflow node that handles waiting for client-answered signal or timeout
// It returns whether to continue to the next node or stop the flow
// This node will wait in a loop for a maximum of 60 seconds if no signal is received
func WaitAnswerWorkflowNode(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("WaitAnswerWorkflowNode: Starting wait_answer - will wait up to 60 seconds for client-answered signal")

	// This node always uses its own fixed timeout (1 minute / 60 seconds)
	// The workflow-level timeoutDuration is for the entire workflow, not individual nodes
	waitAnswerTimeout := 1 * time.Minute
	logger.Info("WaitAnswerWorkflowNode: Waiting for client-answered signal or timeout", "timeout_seconds", int(waitAnswerTimeout.Seconds()))

	// Create channel for client-answered signal
	clientAnsweredChannel := workflow.GetSignalChannel(ctx, "client-answered")

	// Use the node's own start time (when this node starts executing)
	// This ensures the timeout is calculated from when the node starts, not from workflow start
	nodeStartTime := workflow.Now(ctx)

	clientAnswered := false
	var timer workflow.Future
	var cancelTimer workflow.CancelFunc

	// Wait for signal or timeout
	for {
		// Check if client already answered
		if clientAnswered {
			if cancelTimer != nil {
				cancelTimer()
			}
			logger.Info("WaitAnswerWorkflowNode: Client answered, processing")
			break
		}

		// Calculate remaining time from when this node started
		elapsed := workflow.Now(ctx).Sub(nodeStartTime)
		remaining := waitAnswerTimeout - elapsed

		if remaining <= 0 {
			logger.Info("WaitAnswerWorkflowNode: Timeout reached before signal")
			// Update memo to record timeout event
			memo := map[string]interface{}{
				"timeout_occurred": true,
				"timeout_at":       workflow.Now(ctx).UTC(),
				"event":            "timeout",
				"workflow_id":      workflowID,
			}
			if err := workflow.UpsertMemo(ctx, memo); err != nil {
				logger.Error("WaitAnswerWorkflowNode: Failed to upsert memo", "error", err)
			}

			// Return result with activity information - executor will call ExecuteActivity
			return NodeExecutionResult{
				ShouldContinue: true,
				Error:          nil,
				ActivityName:   "wait_answer",
				ClientAnswered: false,
				EventType:      "timeout",
			}
		}

		// Use selector to wait for signal or timeout
		selector := workflow.NewSelector(ctx)

		// Wait for client-answered signal
		selector.AddReceive(clientAnsweredChannel, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, nil) // Receive the signal
			logger.Info("WaitAnswerWorkflowNode: client-answered signal received")
			clientAnswered = true
			if cancelTimer != nil {
				cancelTimer()
			}
		})

		// Add timer if one doesn't already exist
		if timer == nil {
			timerCtx, cancel := workflow.WithCancel(ctx)
			timer = workflow.NewTimer(timerCtx, remaining)
			cancelTimer = cancel

			selector.AddFuture(timer, func(f workflow.Future) {
				cancel()
				logger.Info("WaitAnswerWorkflowNode: Timeout timer fired")
			})
		}

		// Wait for either signal or timeout
		selector.Select(ctx)

		// After Select() returns, check if we should stop
		if clientAnswered {
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
				ShouldContinue: true,
				Error:          nil,
				ActivityName:   "wait_answer",
				ClientAnswered: true,
				EventType:      "client-answered",
			}
		}

		// Check if timer fired (timeout reached)
		if timer != nil && timer.IsReady() {
			logger.Info("WaitAnswerWorkflowNode: Timeout reached")
			// Update memo to record timeout event
			memo := map[string]interface{}{
				"timeout_occurred": true,
				"timeout_at":       workflow.Now(ctx).UTC(),
				"event":            "timeout",
				"workflow_id":      workflowID,
			}
			if err := workflow.UpsertMemo(ctx, memo); err != nil {
				logger.Error("WaitAnswerWorkflowNode: Failed to upsert memo", "error", err)
			}

			// Return result with activity information - executor will call ExecuteActivity
			return NodeExecutionResult{
				ShouldContinue: true,
				Error:          nil,
				ActivityName:   "wait_answer",
				ClientAnswered: false,
				EventType:      "timeout",
			}
		}
	}

	// This should never be reached, but required for compilation
	return NodeExecutionResult{ShouldContinue: false, Error: nil}
}
