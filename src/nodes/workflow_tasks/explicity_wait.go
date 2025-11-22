package workflow_tasks

import (
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

	logger.Info("Sleeping before completion", "duration", waitDuration)

	// Use workflow.Sleep instead of time.After for determinism - this yields to Temporal runtime
	// workflow.Sleep respects context cancellation and ensures determinism across replays
	workflow.Sleep(ctx, waitDuration)
	// Sleep completed normally
	logger.Info("Explicity wait node processed successfully")
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: ExplicitWaitName,
		EventType:    domain.EventTypeConditionSatisfied,
	}

}
