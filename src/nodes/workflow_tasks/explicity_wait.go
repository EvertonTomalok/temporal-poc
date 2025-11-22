package workflow_tasks

import (
	"temporal-poc/src/core/domain"
	"time"

	"go.temporal.io/sdk/workflow"
)

var ExplicitWaitName = "explicity_wait"

func init() {
	// Register node with container (processor and workflow node)
	// No retry policy - pass nil for empty retry policy
	// This is a workflow task because it uses workflow.Sleep for explicit waiting
	// No schema defined for explicity_wait (no input required)
	RegisterNode(ExplicitWaitName, processExplicityWaitNode, nil, NodeTypeWorkflowTask, nil)
}

// processExplicityWaitNode processes the explicity_wait node
// The actual sleep happens here with proper cancellation handling
func processExplicityWaitNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("Processing explicity_wait node", "workflow_id", activityCtx.WorkflowID)

	// TODO: Remove this once we have a proper timeout duration
	timeoutDuration := 15 * time.Second
	logger.Info("Sleeping before completion", "duration", timeoutDuration)

	// Use workflow.Sleep instead of time.After for determinism - this yields to Temporal runtime
	// workflow.Sleep respects context cancellation and ensures determinism across replays
	workflow.Sleep(ctx, timeoutDuration)
	// Sleep completed normally
	logger.Info("Explicity wait node processed successfully")
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: ExplicitWaitName,
		EventType:    domain.EventTypeConditionSatisfied,
	}

}
