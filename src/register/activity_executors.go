package register

import (
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"
)

// ExecuteProcessNodeActivity executes the activity for a specific node
// Uses the node name as the activity name so it appears correctly in the Temporal UI
func ExecuteProcessNodeActivity(ctx workflow.Context, nodeName string, activityCtx ActivityContext) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExecuteProcessNodeActivity: Executing named activity", "node_name", nodeName)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Execute activity using node name as the activity name (appears in UI)
	// The activity must be registered with this exact name in the worker
	// Using string name ensures it appears correctly in Temporal UI
	var result error
	err := workflow.ExecuteActivity(ctx, nodeName, activityCtx).Get(ctx, &result)
	if err != nil {
		logger.Error("ExecuteProcessNodeActivity: Activity execution failed", "node_name", nodeName, "error", err)
		return err
	}
	logger.Info("ExecuteProcessNodeActivity: Activity completed", "node_name", nodeName)
	return result
}

// NodeExecutionResult indicates whether the workflow should continue to the next node or stop
// It also contains information about the activity to execute
type NodeExecutionResult struct {
	ShouldContinue bool
	Error          error
	// Activity information - used by executor to call ExecuteActivity
	ActivityName   string
	ClientAnswered bool
	EventType      string
}

// WorkflowNode is a function type that represents a workflow node
// It returns whether to continue to the next node and any error
type WorkflowNode func(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult

// workflowNodeRegistry is a container that holds registered workflow nodes
type workflowNodeRegistry struct {
	nodes map[string]WorkflowNode
	mu    sync.RWMutex
}

var workflowRegistry = &workflowNodeRegistry{
	nodes: make(map[string]WorkflowNode),
}

// RegisterWorkflowNode registers a workflow node for a given node name
// This is called automatically via init() functions in node files
func RegisterWorkflowNode(nodeName string, node WorkflowNode) {
	workflowRegistry.mu.Lock()
	defer workflowRegistry.mu.Unlock()
	workflowRegistry.nodes[nodeName] = node
}

// ActivityRegistry maintains the order of nodes to be executed dynamically
type ActivityRegistry struct {
	nodeNames []string // Ordered list of node names to execute
}

// NewActivityRegistry creates a new activity registry with the specified node names
// The node names define the execution order (e.g., ["wait_answer", "webhook"])
func NewActivityRegistry(nodeNames ...string) *ActivityRegistry {
	return &ActivityRegistry{
		nodeNames: nodeNames,
	}
}

// Execute orchestrates workflow nodes starting from the first node
// It continues to the next node if the current node indicates to continue, otherwise stops
// All node execution goes through ExecuteActivity - this is the only entry point
func (r *ActivityRegistry) Execute(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow node orchestration", "count", len(r.nodeNames), "nodes", r.nodeNames)

	// Execute nodes in order, starting from the first node
	for i, nodeName := range r.nodeNames {
		logger.Info("Executing node", "node_name", nodeName, "index", i+1, "total", len(r.nodeNames))

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
			logger.Info("Node requested to stop flow", "node_name", nodeName, "remaining_nodes", len(r.nodeNames)-i-1)
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

	// Get the workflow node from registry (workflow nodes handle signal waiting, timeouts, etc.)
	workflowRegistry.mu.RLock()
	workflowNode, exists := workflowRegistry.nodes[nodeName]
	workflowRegistry.mu.RUnlock()

	if !exists {
		logger.Error("Unknown workflow node name", "node_name", nodeName)
		return NodeExecutionResult{
			ShouldContinue: false,
			Error:          nil,
		}, nil
	}

	// Execute the workflow node first (this waits for signals, handles timeouts, etc.)
	logger.Info("ExecuteActivity: Executing workflow node", "node_name", nodeName)
	result := workflowNode(ctx, workflowID, startTime, timeoutDuration, registry)
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
	logger.Info("ExecuteActivity: Calling ExecuteProcessNodeActivity", "node_name", nodeName, "activity_name", activityName)
	if err := ExecuteProcessNodeActivity(ctx, activityName, activityCtx); err != nil {
		logger.Error("Activity execution failed", "node_name", nodeName, "activity_name", activityName, "error", err)
		return result, err
	}

	logger.Info("ExecuteActivity: Node completed", "node_name", nodeName)
	return result, nil
}

// GetNodeOrder returns the current node execution order
func (r *ActivityRegistry) GetNodeOrder() []string {
	return r.nodeNames
}
