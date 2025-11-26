package workflow_tasks

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core/domain"
	"temporal-poc/src/helpers"
	"temporal-poc/src/nodes"
)

var WaitStopSignalName = "wait_stop_signal"

type WaitStopSignalSchema struct {
	TimeoutSeconds int64 `json:"timeout_seconds" jsonschema:"description=Timeout in seconds,required"`
}

func init() {
	// Define schema struct for wait_stop_signal node
	// This struct will be converted to JSON Schema for validation
	schema := &domain.NodeSchema{
		SchemaStruct: WaitStopSignalSchema{},
	}

	// Register node with container (processor and workflow node)
	// This is a workflow task because it waits for signals and uses timers
	// Mark as internal visibility
	RegisterNode(
		WaitStopSignalName,
		waitStopSignalProcessorNode,
		NodeTypeWorkflowTask,
		nodes.WithSchemaWorkflowTask(schema),
		nodes.WithInternalVisibilityWorkflowTask(),
	)
}

// WaitStopSignalWorkflowNode is the workflow node that handles waiting for stop signal or timeout
// It returns whether to continue to the next node or stop the flow
// This node will wait for a configurable timeout (default 60 seconds) if no signal is received
func waitStopSignalProcessorNode(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult {
	logger := workflow.GetLogger(ctx)

	// Get timeout from schema, default to 60 seconds if not provided
	waitStopSignalTimeout := 60 * time.Second
	if schema, err := helpers.UnmarshalSchema[WaitStopSignalSchema](activityCtx.Schema); err == nil {
		if schema.TimeoutSeconds > 0 {
			waitStopSignalTimeout = time.Duration(schema.TimeoutSeconds) * time.Second
		}
	}

	// Create channel for stop signal
	stopChannel := workflow.GetSignalChannel(ctx, domain.StopSignal)

	stopReceived := false
	conditionStatus := ""

	// Create timer outside the loop with summary for UI visibility
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	logger.Info("Creating timer", "node_name", activityCtx.NodeName, "timeout", waitStopSignalTimeout)
	timerFuture := NewTimerWithSummary(timerCtx, waitStopSignalTimeout, "Waiting for stop signal")

	// Create selector outside the loop
	selector := workflow.NewSelector(ctx)

	// Wait for stop signal with payload
	selector.AddReceive(stopChannel, func(c workflow.ReceiveChannel, more bool) {
		var signalPayload domain.StopSignalPayload
		c.Receive(ctx, &signalPayload) // Receive the signal with payload
		logger.Info("WaitStopSignalWorkflowNode: stop signal received",
			"condition_status", signalPayload.ConditionStatus)
		stopReceived = true
		conditionStatus = signalPayload.ConditionStatus
		cancelTimer()
	})

	// Add timer to selector
	selector.AddFuture(timerFuture, func(f workflow.Future) {
		cancelTimer()
		logger.Info("WaitStopSignalWorkflowNode: Timeout timer fired", "node_name", activityCtx.NodeName)
	})

	// Wait for either signal or timeout
	selector.Select(ctx)

	// Check if signal was received
	if stopReceived {
		cancelTimer()
		logger.Info("WaitStopSignalWorkflowNode: Stop signal received, processing",
			"condition_status", conditionStatus)

		// Validate condition_status and determine event type
		var eventType domain.EventType
		switch conditionStatus {
		case "satisfied":
			eventType = domain.EventTypeConditionSatisfied
		case "not_satisfied":
			eventType = domain.EventTypeConditionNotSatisfied
		default:
			// Invalid condition_status, default to not_satisfied
			logger.Warn("WaitStopSignalWorkflowNode: Invalid condition_status, defaulting to not_satisfied",
				"condition_status", conditionStatus)
			eventType = domain.EventTypeConditionNotSatisfied
		}

		logger.Info("WaitStopSignalWorkflowNode: Processing completed", "event_type", eventType)

		return NodeExecutionResult{
			Error:        nil,
			ActivityName: WaitStopSignalName,
			EventType:    eventType,
		}
	}

	// Timer fired (timeout reached)
	logger.Info("WaitStopSignalWorkflowNode: Timeout reached")
	return NodeExecutionResult{
		Error:        nil,
		ActivityName: WaitStopSignalName,
		EventType:    domain.EventTypeConditionTimeout,
	}
}
