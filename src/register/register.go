package register

import (
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"

	"temporal-poc/src/core/domain"
	activities "temporal-poc/src/nodes/activities"
	nodes "temporal-poc/src/nodes/workflow_tasks"
)

// DEPRECATED: This file contains deprecated types kept only for backward compatibility with validation.
// New code should use workflows.WorkflowConfig and workflows.StepConfig instead.

// StepDefinition defines a single step in the workflow
// DEPRECATED: Use workflows.StepConfig instead.
// It can have either a simple "go_to" for linear flow or "conditions" for conditional branching
type StepDefinition struct {
	Node      string            `json:"node"`      // The node name to execute
	GoTo      string            `json:"go_to"`     // Next step for simple linear flow (optional)
	Condition *domain.Condition `json:"condition"` // Conditional branching based on event types (optional)
}

// WorkflowDefinition defines the entire workflow structure with steps and a starter step
// DEPRECATED: Use workflows.WorkflowConfig instead.
// This type is kept only for validation purposes.
type WorkflowDefinition struct {
	Steps     map[string]StepDefinition `json:"steps"`      // Map of step names to step definitions
	StartStep string                    `json:"start_step"` // The starting step name
}

// NodeCaller represents the caller function for a node
// For workflow tasks, this is an ActivityProcessor (workflow context)
// For activities, this is an ActivityFunction (activity context)
type NodeCaller interface {
	// This is a marker interface - actual types are ActivityProcessor or ActivityFunction
}

// NodeInfo holds complete information about a registered node
// This includes both workflow tasks and activities
type NodeInfo struct {
	Name        string                // Node name
	Type        nodes.NodeType        // NodeTypeActivity or NodeTypeWorkflowTask
	Caller      NodeCaller            // Processor for workflow tasks, Function for activities
	RetryPolicy *temporal.RetryPolicy // Retry policy (nil means no retry)
}

// Register is a singleton that provides unified access to all nodes
// It aggregates nodes from both workflow tasks container and activities container
type Register struct {
	workflowTasksContainer *nodes.Container
	activitiesContainer    *activities.Container
	allNodes               map[string]NodeInfo
	mu                     sync.RWMutex
}

var (
	registerInstance *Register
	registerOnce     sync.Once
)

// GetInstance returns the singleton instance of Register
// It aggregates nodes from both workflow tasks and activities containers
func GetInstance() *Register {
	registerOnce.Do(func() {
		registerInstance = &Register{
			workflowTasksContainer: nodes.GetContainer(),
			activitiesContainer:    activities.GetContainer(),
			allNodes:               make(map[string]NodeInfo),
		}
		// Initialize all nodes from both containers
		registerInstance.initializeNodes()
	})
	return registerInstance
}

// initializeNodes populates the allNodes map with nodes from both containers
// This is called once during initialization, but nodes are also loaded on-demand
func (r *Register) initializeNodes() {
	r.refreshNodes()
}

// refreshNodes refreshes the allNodes map from both containers
// This ensures the register always has the latest nodes, even if they're registered after initialization
func (r *Register) refreshNodes() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing nodes
	r.allNodes = make(map[string]NodeInfo)

	// Add workflow tasks
	workflowTaskNames := r.workflowTasksContainer.GetAllNodeNames()
	for _, name := range workflowTaskNames {
		processor, exists := r.workflowTasksContainer.GetProcessor(name)
		if exists {
			r.allNodes[name] = NodeInfo{
				Name:        name,
				Type:        nodes.NodeTypeWorkflowTask,
				Caller:      processor,
				RetryPolicy: r.workflowTasksContainer.GetRetryPolicy(name),
			}
		}
	}

	// Add activities
	activityNames := r.activitiesContainer.GetAllActivityNames()
	for _, name := range activityNames {
		activityFn, exists := r.activitiesContainer.GetActivity(name)
		if exists {
			r.allNodes[name] = NodeInfo{
				Name:        name,
				Type:        nodes.NodeTypeActivity,
				Caller:      activityFn,
				RetryPolicy: nil, // Activities don't have retry policies in the activities container
			}
		}
	}
}

// GetAllNodeNames returns all registered node names (both workflow tasks and activities)
// It checks both containers directly to ensure it always has the latest nodes
func (r *Register) GetAllNodeNames() []string {
	// Get all workflow task names
	workflowTaskNames := r.workflowTasksContainer.GetAllNodeNames()

	// Get all activity names
	activityNames := r.activitiesContainer.GetAllActivityNames()

	// Combine and deduplicate
	nodeMap := make(map[string]bool)
	for _, name := range workflowTaskNames {
		nodeMap[name] = true
	}
	for _, name := range activityNames {
		nodeMap[name] = true
	}

	nodeNames := make([]string, 0, len(nodeMap))
	for name := range nodeMap {
		nodeNames = append(nodeNames, name)
	}
	return nodeNames
}

// GetNodeInfo returns complete information about a node including type and caller
// It checks both containers directly to ensure it always has the latest information
func (r *Register) GetNodeInfo(name string) (NodeInfo, bool) {
	// Check workflow tasks first
	if processor, exists := r.workflowTasksContainer.GetProcessor(name); exists {
		return NodeInfo{
			Name:        name,
			Type:        nodes.NodeTypeWorkflowTask,
			Caller:      processor,
			RetryPolicy: r.workflowTasksContainer.GetRetryPolicy(name),
		}, true
	}

	// Check activities
	if activityFn, exists := r.activitiesContainer.GetActivity(name); exists {
		return NodeInfo{
			Name:        name,
			Type:        nodes.NodeTypeActivity,
			Caller:      activityFn,
			RetryPolicy: nil, // Activities don't have retry policies in the activities container
		}, true
	}

	return NodeInfo{}, false
}

// GetProcessor returns the processor for a given node name (workflow tasks only)
func (r *Register) GetProcessor(name string) (nodes.ActivityProcessor, bool) {
	nodeInfo, exists := r.GetNodeInfo(name)
	if !exists || nodeInfo.Type != nodes.NodeTypeWorkflowTask {
		return nil, false
	}
	processor, ok := nodeInfo.Caller.(nodes.ActivityProcessor)
	return processor, ok
}

// GetWorkflowNode returns the workflow node processor for a given node name
func (r *Register) GetWorkflowNode(name string) (nodes.ActivityProcessor, bool) {
	return r.GetProcessor(name)
}

// GetActivityFunction returns the activity function for a given node name (activities only)
func (r *Register) GetActivityFunction(name string) (activities.ActivityFunction, bool) {
	nodeInfo, exists := r.GetNodeInfo(name)
	if !exists || nodeInfo.Type != nodes.NodeTypeActivity {
		return nil, false
	}
	activityFn, ok := nodeInfo.Caller.(activities.ActivityFunction)
	return activityFn, ok
}

// GetRetryPolicy returns the retry policy for a given node name
func (r *Register) GetRetryPolicy(name string) *temporal.RetryPolicy {
	nodeInfo, exists := r.GetNodeInfo(name)
	if !exists {
		// Return default retry policy if node not found
		return &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 1.1,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    15,
		}
	}
	return nodeInfo.RetryPolicy
}

// IsWorkflowTask returns true if the node is a workflow task (waiter), false if it's an activity task
func (r *Register) IsWorkflowTask(name string) bool {
	nodeInfo, exists := r.GetNodeInfo(name)
	if !exists {
		return false
	}
	return nodeInfo.Type == nodes.NodeTypeWorkflowTask
}

// Convenience functions that use the singleton instance

// GetAllNodeNames returns all registered node names (both workflow tasks and activities)
func GetAllNodeNames() []string {
	return GetInstance().GetAllNodeNames()
}

// GetNodeInfo returns complete information about a node including type and caller
func GetNodeInfo(name string) (NodeInfo, bool) {
	return GetInstance().GetNodeInfo(name)
}

// GetProcessor returns the processor for a given node name (workflow tasks only)
func GetProcessor(name string) (nodes.ActivityProcessor, bool) {
	return GetInstance().GetProcessor(name)
}

// GetWorkflowNode returns the workflow node processor for a given node name
func GetWorkflowNode(name string) (nodes.ActivityProcessor, bool) {
	return GetInstance().GetWorkflowNode(name)
}

// GetActivityFunction returns the activity function for a given node name (activities only)
func GetActivityFunction(name string) (activities.ActivityFunction, bool) {
	return GetInstance().GetActivityFunction(name)
}

// GetRetryPolicy returns the retry policy for a given node name
func GetRetryPolicy(name string) *temporal.RetryPolicy {
	return GetInstance().GetRetryPolicy(name)
}

// IsWorkflowTask returns true if the node is a workflow task (waiter), false if it's an activity task
func IsWorkflowTask(name string) bool {
	return GetInstance().IsWorkflowTask(name)
}

// Re-export node types for convenience
var (
	NodeTypeActivity     = nodes.NodeTypeActivity
	NodeTypeWorkflowTask = nodes.NodeTypeWorkflowTask
)
