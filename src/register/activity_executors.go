package register

import (
	"temporal-poc/src/nodes"
	"time"

	"go.temporal.io/sdk/workflow"
)

// ExecuteProcessNodeActivity executes the activity for a specific node
// Uses the node name as the activity name so it appears correctly in the Temporal UI
// Activities don't have timers - timers are only in workflow nodes
// timeoutDuration is used to set the activity timeout - nodes handle their own timeout logic
func ExecuteProcessNodeActivity(ctx workflow.Context, nodeName string, activityCtx ActivityContext, timeoutDuration time.Duration) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExecuteProcessNodeActivity: Executing named activity", "node_name", nodeName)

	// Set activity timeout - activities should have reasonable timeouts (max 10 minutes)
	// Use a reasonable default if timeoutDuration is too large or invalid
	activityTimeout := 5 * time.Minute // Default to 5 minutes for activities
	if timeoutDuration > 0 && timeoutDuration < 10*time.Minute {
		// Use the provided timeout if it's reasonable (less than 10 minutes)
		activityTimeout = timeoutDuration
	}

	// Set activity options with timeout before executing
	// Activities require StartToCloseTimeout to be set
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: activityTimeout,
	}
	activityCtxWithOptions := workflow.WithActivityOptions(ctx, ao)

	// Execute activity using node name as the activity name (appears in UI)
	// The activity must be registered with this exact name in the worker
	// Using string name ensures it appears correctly in Temporal UI
	// Use activityCtxWithOptions for ExecuteActivity to ensure timeout is applied
	var result error
	err := workflow.ExecuteActivity(activityCtxWithOptions, nodeName, activityCtx).Get(ctx, &result)
	if err != nil {
		logger.Error("ExecuteProcessNodeActivity: Activity execution failed", "node_name", nodeName, "error", err)
		return err
	}
	logger.Info("ExecuteProcessNodeActivity: Activity completed", "node_name", nodeName)
	return result
}

// NodeExecutionResult is an alias for nodes.NodeExecutionResult
type NodeExecutionResult = nodes.NodeExecutionResult

// WorkflowNode is an alias for nodes.WorkflowNode
type WorkflowNode = nodes.WorkflowNode

// ActivityRegistry wraps nodes.ActivityRegistry to add methods
type ActivityRegistry struct {
	*nodes.ActivityRegistry
}

// RegisterWorkflowNode registers a workflow node for a given node name
// DEPRECATED: Nodes now register directly with container
func RegisterWorkflowNode(nodeName string, node WorkflowNode) {
	// This is now a no-op as nodes register directly with container
}

// NewActivityRegistry creates a new activity registry with the specified node names
// The node names define the execution order (e.g., ["wait_answer", "webhook"])
func NewActivityRegistry(nodeNames ...string) *ActivityRegistry {
	return &ActivityRegistry{
		ActivityRegistry: &nodes.ActivityRegistry{
			NodeNames: nodeNames,
		},
	}
}

// Execute orchestrates workflow nodes starting from the first node
// It continues to the next node if the current node indicates to continue, otherwise stops
// All node execution goes through ExecuteActivity - this is the only entry point
func (r *ActivityRegistry) Execute(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow node orchestration", "count", len(r.NodeNames), "nodes", r.NodeNames)

	// Execute nodes in order, starting from the first node
	for i, nodeName := range r.NodeNames {
		logger.Info("Executing node", "node_name", nodeName, "index", i+1, "total", len(r.NodeNames))

		// ExecuteActivity is the only entry point for executing nodes
		// It handles both workflow node execution and activity execution
		result, err := ExecuteActivity(ctx, nodeName, workflowID, startTime, timeoutDuration, r)
		if err != nil {
			logger.Error("Node execution failed", "node_name", nodeName, "index", i+1, "error", err)
			return err
		}

		logger.Info("Node completed", "node_name", nodeName, "index", i+1, "should_continue", result.ShouldContinue)

		// If node indicates to stop, end the flow immediately
		// This respects the ShouldContinue flag and stops remaining nodes from executing
		if !result.ShouldContinue {
			logger.Info("Node requested to stop flow", "node_name", nodeName, "remaining_nodes", len(r.NodeNames)-i-1)
			return nil
		}

		// Continue to next node
	}

	logger.Info("All workflow nodes completed")
	return nil
}

// ExecuteActivity is the ONLY entry point for executing nodes
// It first executes the workflow node (for signal waiting, etc.), then ExecuteProcessNodeActivity for the activity
// ExecuteProcessNodeActivity is the only function allowed to run activity processors
func ExecuteActivity(ctx workflow.Context, nodeName string, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) (NodeExecutionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExecuteActivity: Executing node", "node_name", nodeName)

	// Get the workflow node from nodes (workflow nodes handle signal waiting, timeouts, etc.)
	workflowNode, exists := nodes.GetWorkflowNode(nodeName)

	if !exists {
		logger.Error("Unknown workflow node name", "node_name", nodeName)
		return NodeExecutionResult{
			ShouldContinue: false,
			Error:          nil,
		}, nil
	}

	// Execute the workflow node first (this waits for signals, handles timeouts, etc.)
	logger.Info("ExecuteActivity: Executing workflow node", "node_name", nodeName)
	result := workflowNode(ctx, workflowID, startTime, timeoutDuration, registry.ActivityRegistry)
	if result.Error != nil {
		logger.Error("Workflow node execution failed", "node_name", nodeName, "error", result.Error)
		return result, result.Error
	}

	// After workflow node completes, execute the activity processor via ExecuteProcessNodeActivity
	// ExecuteProcessNodeActivity is the only function allowed to run activity processors
	activityName := result.ActivityName
	if activityName == "" {
		activityName = nodeName
	}

	// Build activity context from workflow node result
	activityCtx := ActivityContext{
		WorkflowID:      workflowID,
		ClientAnswered:  result.ClientAnswered,
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
		EventTime:       workflow.Now(ctx),
		EventType:       result.EventType,
	}

	// Execute the activity processor - ExecuteProcessNodeActivity is the only function allowed to run activity processors
	// Activities don't have timers - timers are only in workflow nodes
	// Pass timeoutDuration to the activity executor - nodes handle their own timeout logic
	logger.Info("ExecuteActivity: Calling ExecuteProcessNodeActivity", "node_name", nodeName, "activity_name", activityName)
	if err := ExecuteProcessNodeActivity(ctx, activityName, activityCtx, timeoutDuration); err != nil {
		logger.Error("Activity execution failed", "node_name", nodeName, "activity_name", activityName, "error", err)
		return result, err
	}

	logger.Info("ExecuteActivity: Node completed", "node_name", nodeName)
	return result, nil
}

// GetNodeOrder returns the current node execution order
func (r *ActivityRegistry) GetNodeOrder() []string {
	return r.NodeNames
}
