package register

import (
	"context"

	"temporal-poc/src/nodes"

	"go.temporal.io/sdk/activity"
)

// ActivityContext is an alias for nodes.ActivityContext
type ActivityContext = nodes.ActivityContext

// ActivityProcessor is an alias for nodes.ActivityProcessor
type ActivityProcessor = nodes.ActivityProcessor

// Register is a singleton struct for managing activity registration
type Register struct{}

var instance = &Register{}

// GetInstance returns the singleton instance of Register
func GetInstance() *Register {
	return instance
}

// GetNamedActivityFunction returns an activity function for a specific node name
// This allows the activity to appear with the node name in the Temporal UI
// The nodeName is captured in the closure to ensure the correct processor is called
func GetNamedActivityFunction(nodeName string) func(context.Context, ActivityContext) error {
	// Capture nodeName in the closure to avoid closure variable issues
	capturedNodeName := nodeName
	return func(ctx context.Context, activityCtx ActivityContext) error {
		logger := activity.GetLogger(ctx)
		logger.Info("Processing node", "node_name", capturedNodeName, "workflow_id", activityCtx.WorkflowID)

		processor, exists := nodes.GetProcessor(capturedNodeName)
		if !exists {
			logger.Error("Unknown node name", "node_name", capturedNodeName)
			return nil // Don't fail workflow for unknown nodes
		}

		return processor(ctx, activityCtx)
	}
}

// GetAllRegisteredNodeNames returns all registered node names
// This is used to register all named activities in the worker
// It gets the names from the nodes container to avoid circular imports
func GetAllRegisteredNodeNames() []string {
	return nodes.GetAllNodeNames()
}
