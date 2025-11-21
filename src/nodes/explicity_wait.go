package nodes

import (
	"context"
	"temporal-poc/src/core"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

var ExplicitWaitName = "explicity_wait"

func init() {
	// Register node with container (processor and workflow node)
	RegisterNode(ExplicitWaitName, processExplicityWaitNode, ExplicityWaitWorkflowNode)
}

// processExplicityWaitNode processes the explicity_wait node
// The actual sleep happens here with proper cancellation handling
func processExplicityWaitNode(ctx context.Context, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing explicity_wait node", "workflow_id", activityCtx.WorkflowID)

	// TODO: Remove this once we have a proper timeout duration
	timeoutDuration := 15 * time.Second
	logger.Info("Sleeping before completion", "duration", timeoutDuration)
	// Use select to respect context cancellation (Temporal activity cancellation)
	select {
	case <-time.After(timeoutDuration):
		// Sleep completed normally
	case <-ctx.Done():
		// Activity was cancelled, return the cancellation error
		logger.Info("Activity cancelled during sleep")
		return ctx.Err()
	}

	logger.Info("Explicity wait node processed successfully")
	return nil
}

// ExplicityWaitWorkflowNode is the workflow node that handles explicitly waiting for a period of time
// It orchestrates the wait by executing the activity, then continues to the next node
func ExplicityWaitWorkflowNode(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExplicityWaitWorkflowNode: Orchestrating explicit wait")

	// The actual work (sleeping) is done in the activity
	// This workflow node just orchestrates and returns the result
	// The activity will be executed by ExecuteActivity after this returns

	logger.Info("ExplicityWaitWorkflowNode: Ready to execute activity, continuing to next node")
	// Return result with activity information - executor will call ExecuteActivity
	// Continue to next node after activity completes
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: ExplicitWaitName,
		EventType:    core.EventTypeSatisfied,
	}
}
