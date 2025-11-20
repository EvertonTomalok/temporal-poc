package nodes

import (
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"
)

// ExecuteProcessNodeActivity executes the ProcessNodeActivity for a specific node
func ExecuteProcessNodeActivity(ctx workflow.Context, nodeName string, activityCtx ActivityContext) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result error
	err := workflow.ExecuteActivity(ctx, ProcessNodeActivity, nodeName, activityCtx).Get(ctx, &result)
	if err != nil {
		return err
	}
	return result
}

// NodeExecutionResult indicates whether the workflow should continue to the next node or stop
type NodeExecutionResult struct {
	ShouldContinue bool
	Error          error
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
func (r *ActivityRegistry) Execute(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow node orchestration", "count", len(r.nodeNames), "nodes", r.nodeNames)

	// Execute nodes in order, starting from the first node
	for i, nodeName := range r.nodeNames {
		logger.Info("Executing workflow node", "node_name", nodeName, "index", i+1, "total", len(r.nodeNames))

		// Get the workflow node from registry
		workflowRegistry.mu.RLock()
		workflowNode, exists := workflowRegistry.nodes[nodeName]
		workflowRegistry.mu.RUnlock()

		if !exists {
			logger.Error("Unknown workflow node name", "node_name", nodeName)
			// Continue to next node if unknown (don't fail workflow)
			continue
		}

		// Execute the workflow node
		result := workflowNode(ctx, workflowID, startTime, timeoutDuration, r)
		if result.Error != nil {
			logger.Error("Workflow node execution failed", "node_name", nodeName, "index", i+1, "error", result.Error)
			return result.Error
		}

		logger.Info("Workflow node completed", "node_name", nodeName, "index", i+1, "should_continue", result.ShouldContinue)

		// If node indicates to stop, end the flow
		if !result.ShouldContinue {
			logger.Info("Workflow node requested to stop flow", "node_name", nodeName)
			return nil
		}

		// Continue to next node
	}

	logger.Info("All workflow nodes completed")
	return nil
}

// ExecuteActivities executes activity nodes in order using the workflow context
// This is used by workflow nodes to execute their associated activities
func (r *ActivityRegistry) ExecuteActivities(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, clientAnswered bool, eventType string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing activity nodes in order", "count", len(r.nodeNames), "nodes", r.nodeNames)

	// Build activity context
	activityCtx := ActivityContext{
		WorkflowID:      workflowID,
		ClientAnswered:  clientAnswered,
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
		EventTime:       workflow.Now(ctx),
		EventType:       eventType,
	}

	// Execute each activity node in order
	for i, nodeName := range r.nodeNames {
		logger.Info("Executing activity node", "node_name", nodeName, "index", i+1, "total", len(r.nodeNames))
		if err := ExecuteProcessNodeActivity(ctx, nodeName, activityCtx); err != nil {
			logger.Error("Activity node execution failed", "node_name", nodeName, "index", i+1, "error", err)
			return err
		}
		logger.Info("Activity node completed", "node_name", nodeName, "index", i+1)
	}

	return nil
}

// GetNodeOrder returns the current node execution order
func (r *ActivityRegistry) GetNodeOrder() []string {
	return r.nodeNames
}
