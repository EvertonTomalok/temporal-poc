package workflow_tasks

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/helpers"
	nodes "temporal-poc/src/nodes"
)

var WaitAnswerName = "wait_answer"

type WaitAnswerSchema struct {
	TimeoutSeconds int64 `json:"timeout_seconds" jsonschema:"description=Timeout in seconds,required"`
}

func init() {
	// Define schema struct for wait_answer node
	// This struct will be converted to JSON Schema for validation
	schema := &domain.NodeSchema{
		SchemaStruct: WaitAnswerSchema{},
	}

	// Register node with container (processor and workflow node)
	// This is a workflow task because it waits for signals and uses timers
	RegisterNode(
		WaitAnswerName,
		waitAnswerProcessorNode,
		NodeTypeWorkflowTask,
		nodes.WithSchemaWorkflowTask(schema),
		nodes.WithPublicVisibilityWorkflowTask(),
	)
}

// WaitAnswerWorkflowNode is the workflow node that handles waiting for client-answered signal or timeout
// It returns whether to continue to the next node or stop the flow
// This node will wait for a configurable timeout (default 30 seconds) if no signal is received
func waitAnswerProcessorNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)

	// Get timeout from schema, default to 30 seconds if not provided
	waitAnswerTimeout := 60 * time.Second
	if schema, err := helpers.UnmarshalSchema[WaitAnswerSchema](activityCtx.Schema); err == nil {
		if schema.TimeoutSeconds > 0 {
			waitAnswerTimeout = time.Duration(schema.TimeoutSeconds) * time.Second
		}
	}

	// Create channel for client-answered signal
	clientAnsweredChannel := workflow.GetSignalChannel(ctx, domain.ClientAnsweredSignal)

	clientAnswered := false

	// Create timer outside the loop with summary for UI visibility
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	logger.Info("Creating timer", "node_name", activityCtx.NodeName, "timeout", waitAnswerTimeout)
	timerFuture := NewTimerWithSummary(timerCtx, waitAnswerTimeout, "Waiting for answer")

	// Create selector outside the loop
	selector := workflow.NewSelector(ctx)

	// Wait for client-answered signal
	var signalPayload domain.ClientAnsweredSignalPayload
	selector.AddReceive(clientAnsweredChannel, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &signalPayload) // Receive the signal with payload
		logger.Info("WaitAnswerWorkflowNode: client-answered signal received", "message", signalPayload.Message)
		clientAnswered = true
		cancelTimer()
	})

	// Add timer to selector
	selector.AddFuture(timerFuture, func(f workflow.Future) {
		cancelTimer()
		logger.Info("WaitAnswerWorkflowNode: Timeout timer fired", "node_name", activityCtx.NodeName)
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
			Metadata:     map[string]interface{}{"message": signalPayload.Message},
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
