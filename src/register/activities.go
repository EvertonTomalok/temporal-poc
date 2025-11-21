package register

import (
	"context"
	"temporal-poc/src/nodes"
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

// ProcessNodeActivity is a generic activity function that processes any node
// The actual processor (which expects workflow.Context) is called from the workflow node,
// not from the activity. This activity is a placeholder for UI display purposes.
// Note: Activities use context.Context, not workflow.Context
func ProcessNodeActivity(ctx context.Context, activityCtx ActivityContext) error {
	// Note: The actual processor is called from the workflow node (in ExecuteActivity),
	// not from here. This activity is registered for UI display purposes.
	// The processor expects workflow.Context which is only available in workflows.
	_ = ctx
	_ = activityCtx
	return nil
}

// GetAllRegisteredNodeNames returns all registered node names
// This is used to register all named activities in the worker
// It gets the names from the nodes container to avoid circular imports
func GetAllRegisteredNodeNames() []string {
	return nodes.GetAllNodeNames()
}
