package nodes

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

func init() {
	// Auto-register the wait_answer activity
	RegisterActivityProcessor("wait_answer", processClientAnsweredProcessorNode)
	// Auto-register the wait_answer workflow node
	RegisterWorkflowNode("wait_answer", WaitAnswerWorkflowNode)
}

// processClientAnsweredProcessorNode processes the client-answered processor node
func processClientAnsweredProcessorNode(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing client-answered processor node", "workflow_id", activityCtx.WorkflowID)

	// This activity processes the client-answered event
	// Note: Search attributes must be updated from workflow context, not activity context
	// So we'll need to return information that the workflow can use to update search attributes
	logger.Info("Client-answered processor node processed successfully")

	return nil
}

// WaitAnswerWorkflowNode is the workflow node that handles waiting for client-answered signal or timeout
// It returns whether to continue to the next node or stop the flow
func WaitAnswerWorkflowNode(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("WaitAnswerWorkflowNode: Waiting for client-answered signal or timeout")

	// Create channel for client-answered signal
	clientAnsweredChannel := workflow.GetSignalChannel(ctx, "client-answered")

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

		// Calculate remaining time
		elapsed := workflow.Now(ctx).Sub(startTime)
		remaining := timeoutDuration - elapsed

		if remaining <= 0 {
			logger.Info("WaitAnswerWorkflowNode: Timeout reached before signal")
			// Process timeout event - execute activities and continue to next node
			if err := registry.ExecuteActivities(ctx, workflowID, startTime, timeoutDuration, false, "timeout"); err != nil {
				logger.Error("WaitAnswerWorkflowNode: Failed to execute timeout activities", "error", err)
				return NodeExecutionResult{ShouldContinue: false, Error: err}
			}

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

			// Continue to next node after timeout
			return NodeExecutionResult{ShouldContinue: true, Error: nil}
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
			// Execute activities for client-answered event
			if err := registry.ExecuteActivities(ctx, workflowID, startTime, timeoutDuration, true, "client-answered"); err != nil {
				logger.Error("WaitAnswerWorkflowNode: Failed to execute client-answered activities", "error", err)
				return NodeExecutionResult{ShouldContinue: false, Error: err}
			}

			// Update search attributes from workflow context
			err := workflow.UpsertTypedSearchAttributes(
				ctx,
				ClientAnsweredField.ValueSet(true),
				ClientAnsweredAtField.ValueSet(workflow.Now(ctx).UTC()),
			)
			if err != nil {
				logger.Error("WaitAnswerWorkflowNode: Failed to upsert search attributes", "error", err)
			} else {
				logger.Info("WaitAnswerWorkflowNode: Successfully upserted client-answered search attributes")
			}

			logger.Info("WaitAnswerWorkflowNode: Processing completed")
			// Continue to next node after client answered
			return NodeExecutionResult{ShouldContinue: true, Error: nil}
		}

		// Check if timer fired (timeout reached)
		if timer != nil && timer.IsReady() {
			logger.Info("WaitAnswerWorkflowNode: Timeout reached")
			// Process timeout event - execute activities and continue to next node
			if err := registry.ExecuteActivities(ctx, workflowID, startTime, timeoutDuration, false, "timeout"); err != nil {
				logger.Error("WaitAnswerWorkflowNode: Failed to execute timeout activities", "error", err)
				return NodeExecutionResult{ShouldContinue: false, Error: err}
			}

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

			// Continue to next node after timeout
			return NodeExecutionResult{ShouldContinue: true, Error: nil}
		}
	}

	// This should never be reached, but required for compilation
	return NodeExecutionResult{ShouldContinue: false, Error: nil}
}

// ClientAnsweredNode handles waiting for client-answered signal and processing it
// This function contains all the logic for handling client-answered events and timeout events
// DEPRECATED: Use WaitAnswerWorkflowNode instead
func ClientAnsweredNode(ctx workflow.Context, startTime time.Time, timeoutDuration time.Duration, workflowID string, registry *ActivityRegistry) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ClientAnsweredNode: Waiting for client-answered signal or timeout")

	// Create channel for client-answered signal
	clientAnsweredChannel := workflow.GetSignalChannel(ctx, "client-answered")

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
			logger.Info("ClientAnsweredNode: Client answered, processing")
			break
		}

		// Calculate remaining time
		elapsed := workflow.Now(ctx).Sub(startTime)
		remaining := timeoutDuration - elapsed

		if remaining <= 0 {
			logger.Info("ClientAnsweredNode: Timeout reached before signal")
			// Process timeout event
			if err := TimeoutWebhookNode(ctx, workflowID, startTime, timeoutDuration, registry); err != nil {
				logger.Error("ClientAnsweredNode: Error in TimeoutWebhookNode", "error", err)
				return err
			}
			return nil // Timeout processed successfully
		}

		// Use selector to wait for signal or timeout
		selector := workflow.NewSelector(ctx)

		// Wait for client-answered signal
		selector.AddReceive(clientAnsweredChannel, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, nil) // Receive the signal
			logger.Info("ClientAnsweredNode: client-answered signal received")
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
				logger.Info("ClientAnsweredNode: Timeout timer fired")
			})
		}

		// Wait for either signal or timeout
		selector.Select(ctx)

		// After Select() returns, check if we should stop
		if clientAnswered {
			// Execute activities in order (registry handles execution internally)
			if err := registry.ExecuteActivities(ctx, workflowID, startTime, timeoutDuration, true, "client-answered"); err != nil {
				logger.Error("ClientAnsweredNode: Failed to execute activities", "error", err)
				return err
			}

			// Update search attributes from workflow context
			err := workflow.UpsertTypedSearchAttributes(
				ctx,
				ClientAnsweredField.ValueSet(true),
				ClientAnsweredAtField.ValueSet(workflow.Now(ctx).UTC()),
			)
			if err != nil {
				logger.Error("ClientAnsweredNode: Failed to upsert search attributes", "error", err)
			} else {
				logger.Info("ClientAnsweredNode: Successfully upserted client-answered search attributes")
			}

			logger.Info("ClientAnsweredNode: Processing completed")
			return nil // Client answered and processed successfully
		}

		// Check if timer fired (timeout reached)
		if timer != nil && timer.IsReady() {
			logger.Info("ClientAnsweredNode: Timeout reached")
			// Process timeout event
			if err := TimeoutWebhookNode(ctx, workflowID, startTime, timeoutDuration, registry); err != nil {
				logger.Error("ClientAnsweredNode: Error in TimeoutWebhookNode", "error", err)
				return err
			}
			return nil // Timeout processed successfully
		}
	}

	// This should never be reached, but required for compilation
	// If clientAnswered was true, we should have processed it and returned above
	return nil
}
