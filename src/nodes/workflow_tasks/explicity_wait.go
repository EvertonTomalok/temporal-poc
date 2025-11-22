package workflow_tasks

import (
	"fmt"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/helpers"
	"time"

	"go.temporal.io/sdk/workflow"
)

var ExplicitWaitName = "explicity_wait"

type ExplicityWaitSchema struct {
	WaitSeconds int64 `json:"wait_seconds" jsonschema:"description=Wait in seconds,required"`
}

func init() {
	explicity_wait_schema := &domain.NodeSchema{
		SchemaStruct: ExplicityWaitSchema{},
	}
	RegisterNode(ExplicitWaitName, processExplicityWaitNode, nil, NodeTypeWorkflowTask, explicity_wait_schema)
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
	// Equivalent to Java: Workflow.newTimer(Duration.ofSeconds(2), TimerOptions.newBuilder().setSummary("my-timer").build())
	timerSummary := fmt.Sprintf("%s-wait", activityCtx.NodeName)
	timerOptions := workflow.TimerOptions{
		Summary: timerSummary,
	}
	logger.Info("Creating timer for explicit wait", "node_name", activityCtx.NodeName, "duration", waitDuration, "summary", timerSummary)

	// Use NewTimerWithOptions instead of Sleep to set timer summary for UI visibility
	// This creates a named timer that will be visible in the Temporal UI
	timer := workflow.NewTimerWithOptions(ctx, waitDuration, timerOptions)
	err := timer.Get(ctx, nil)
	if err != nil {
		logger.Error("Timer was canceled", "error", err, "timer_name", timerSummary, "node_name", activityCtx.NodeName)
		return NodeExecutionResult{
			Error:        err,
			ActivityName: ExplicitWaitName,
			EventType:    domain.EventTypeConditionSatisfied,
		}
	}
	// Timer completed successfully
	logger.Info("Timer completed successfully", "timer_name", timerSummary, "node_name", activityCtx.NodeName, "duration", waitDuration)
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: ExplicitWaitName,
		EventType:    domain.EventTypeConditionSatisfied,
	}

}
