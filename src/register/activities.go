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

// Register is kept for backward compatibility but now delegates to container
// DEPRECATED: Use container directly instead
type Register struct{}

var instance = &Register{}

// GetInstance returns the singleton instance of Register
func GetInstance() *Register {
	return instance
}

// RegisterActivityProcessor registers an activity processor for a given node name
// DEPRECATED: Nodes now register directly with container
func (r *Register) RegisterActivityProcessor(nodeName string, processor ActivityProcessor) {
	// This is now a no-op as nodes register directly with container
}

// ProcessNodeActivity is a generic activity that processes a node
// The nodeName identifies which node to process (e.g., "client_answered_processor", "timeout_webhook")
// DEPRECATED: Use GetNamedActivityFunction instead to get activities with node names for UI display
func (r *Register) ProcessNodeActivity(ctx context.Context, nodeName string, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("ProcessNodeActivity: Processing node", "node_name", nodeName, "workflow_id", activityCtx.WorkflowID)

	processor, exists := nodes.GetProcessor(nodeName)
	if !exists {
		logger.Error("Unknown node name", "node_name", nodeName)
		return nil // Don't fail workflow for unknown nodes
	}

	return processor(ctx, activityCtx)
}

// GetNamedActivityFunction returns an activity function for a specific node name
// This allows the activity to appear with the node name in the Temporal UI
// The nodeName is captured in the closure to ensure the correct processor is called
func (r *Register) GetNamedActivityFunction(nodeName string) func(context.Context, ActivityContext) error {
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

// AllProcessorsNames returns a slice of strings containing all registered processor names
func (r *Register) AllProcessorsNames() []string {
	return nodes.GetAllNodeNames()
}

// Legacy functions for backward compatibility
// These delegate to the singleton instance

// RegisterActivityProcessor registers an activity processor for a given node name
// DEPRECATED: Nodes now register directly with container
func RegisterActivityProcessor(nodeName string, processor ActivityProcessor) {
	// This is now a no-op as nodes register directly with container
}

// ProcessNodeActivity is a generic activity that processes a node
// The nodeName identifies which node to process (e.g., "client_answered_processor", "timeout_webhook")
// DEPRECATED: Use GetNamedActivityFunction instead to get activities with node names for UI display
func ProcessNodeActivity(ctx context.Context, nodeName string, activityCtx ActivityContext) error {
	return GetInstance().ProcessNodeActivity(ctx, nodeName, activityCtx)
}

// GetNamedActivityFunction returns an activity function for a specific node name
// This allows the activity to appear with the node name in the Temporal UI
// The nodeName is captured in the closure to ensure the correct processor is called
func GetNamedActivityFunction(nodeName string) func(context.Context, ActivityContext) error {
	return GetInstance().GetNamedActivityFunction(nodeName)
}

// GetAllRegisteredNodeNames returns all registered node names
// This is used to register all named activities in the worker
// It gets the names from the nodes container to avoid circular imports
func GetAllRegisteredNodeNames() []string {
	return nodes.GetAllNodeNames()
}
