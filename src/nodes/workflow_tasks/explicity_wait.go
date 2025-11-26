package workflow_tasks

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core/domain"
	"temporal-poc/src/helpers"
)

var ExplicitWaitName = "explicity_wait"

type ExplicityWaitSchema struct {
	WaitSeconds int64 `json:"wait_seconds" jsonschema:"description=Wait in seconds,required"`
}

func init() {
	explicity_wait_schema := &domain.NodeSchema{
		SchemaStruct: ExplicityWaitSchema{},
	}
	RegisterNode(
		ExplicitWaitName,
		processExplicityWaitNode,
		WithSchemaWorkflowTask(explicity_wait_schema),
		WithPublicVisibilityWorkflowTask(),
	)
}

// processExplicityWaitNode processes the explicity_wait node
// The actual sleep happens here with proper cancellation handling
func processExplicityWaitNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)
	logger.Info("Processing explicity_wait node", "workflow_id", activityCtx.WorkflowID)

	// Get wait duration from schema, default to 15 seconds if not provided
	waitDuration := 15 * time.Second
	if schema, err := helpers.UnmarshalSchema[ExplicityWaitSchema](activityCtx.Schema); err == nil {
		if schema.WaitSeconds > 0 {
			waitDuration = time.Duration(schema.WaitSeconds) * time.Second
		}
	}

	// Create timer with summary for UI visibility
	logger.Info("Creating timer for explicit wait", "node_name", activityCtx.NodeName, "duration", waitDuration)

	// Use NewTimerWithSummary helper to create a timer with summary for UI visibility
	// This creates a named timer that will be visible in the Temporal UI
	timerFuture := NewTimerWithSummary(ctx, waitDuration, fmt.Sprintf("Explicity Wait %s", waitDuration))
	err := timerFuture.Get(ctx, nil)
	if err != nil {
		logger.Error("Timer was canceled", "error", err, "node_name", activityCtx.NodeName)
		return NodeExecutionResult{
			Error:        err,
			ActivityName: ExplicitWaitName,
			EventType:    domain.EventTypeConditionSatisfied,
		}
	}
	// Timer completed successfully
	logger.Info("Timer completed successfully", "node_name", activityCtx.NodeName, "duration", waitDuration)
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: ExplicitWaitName,
		EventType:    domain.EventTypeConditionSatisfied,
	}

}
