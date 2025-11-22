package workflow_tasks

import (
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"

	"temporal-poc/src/core/domain"
	"temporal-poc/src/nodes/activities"
)

// Re-export activity-related types for convenience
// These types are defined in the activities package
type (
	ActivityContext     = activities.ActivityContext
	ActivityProcessor   = activities.ActivityProcessor
	NodeExecutionResult = activities.NodeExecutionResult
)

// NodeType indicates whether a node is a workflow task or activity task
type NodeType int

const (
	// NodeTypeActivity indicates the node should execute as an activity task
	NodeTypeActivity NodeType = iota
	// NodeTypeWorkflowTask indicates the node should execute as a workflow task (waiter)
	// Workflow tasks use workflow.Sleep, timers, or signals and execute directly in workflow
	NodeTypeWorkflowTask
)

// WorkflowTaskInfo holds information about a registered workflow task
type WorkflowTaskInfo struct {
	Name        string
	Processor   ActivityProcessor
	RetryPolicy *temporal.RetryPolicy // Retry policy for the node (nil means no retry)
	Schema      *domain.NodeSchema    // Input schema for the node (optional)
}

// Container holds all registered workflow tasks with their processors
// This container only handles workflow tasks, not activities
type Container struct {
	workflowTasks map[string]WorkflowTaskInfo
	mu            sync.RWMutex
}

var (
	containerInstance *Container
	containerOnce     sync.Once
)

// GetContainer returns the singleton instance of Container
func GetContainer() *Container {
	containerOnce.Do(func() {
		containerInstance = &Container{
			workflowTasks: make(map[string]WorkflowTaskInfo),
		}
	})
	return containerInstance
}

// RegisterNode registers a workflow task node name and processor in the container
// This is called by each workflow task node's init() function
// If retryPolicy is nil, no retry policy will be applied (empty retry policy)
// If schema is nil, no schema validation will be performed for this node
// This container only accepts workflow tasks (NodeTypeWorkflowTask)
func RegisterNode(name string, processor ActivityProcessor, retryPolicy *temporal.RetryPolicy, nodeType NodeType, schema *domain.NodeSchema) {
	if nodeType != NodeTypeWorkflowTask {
		// This container only handles workflow tasks
		return
	}
	container := GetContainer()
	container.mu.Lock()
	defer container.mu.Unlock()
	container.workflowTasks[name] = WorkflowTaskInfo{
		Name:        name,
		Processor:   processor,
		RetryPolicy: retryPolicy,
		Schema:      schema,
	}
}

// GetProcessor returns the processor for a given workflow task node name
func (c *Container) GetProcessor(name string) (ActivityProcessor, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	taskInfo, exists := c.workflowTasks[name]
	if !exists {
		return nil, false
	}
	return taskInfo.Processor, true
}

// GetAllNodeNames returns all registered workflow task node names
func (c *Container) GetAllNodeNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodeNames := make([]string, 0, len(c.workflowTasks))
	for name := range c.workflowTasks {
		nodeNames = append(nodeNames, name)
	}
	return nodeNames
}

// GetAllNodeNames is a convenience function that returns all registered workflow task node names
func GetAllNodeNames() []string {
	return GetContainer().GetAllNodeNames()
}

// GetProcessor is a convenience function that returns the processor for a workflow task node name
func GetProcessor(name string) (ActivityProcessor, bool) {
	return GetContainer().GetProcessor(name)
}

// GetWorkflowNode returns the workflow node processor for a given node name
func (c *Container) GetWorkflowNode(name string) (ActivityProcessor, bool) {
	return c.GetProcessor(name)
}

// GetWorkflowNode is a convenience function that returns the workflow node processor for a node name
func GetWorkflowNode(name string) (ActivityProcessor, bool) {
	return GetContainer().GetWorkflowNode(name)
}

// GetRetryPolicy returns the retry policy for a given workflow task node name
// If no retry policy is configured, return a default retry policy
func (c *Container) GetRetryPolicy(name string) *temporal.RetryPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	taskInfo, exists := c.workflowTasks[name]
	if !exists {
		return &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 1.1,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    15,
		}
	}
	return taskInfo.RetryPolicy
}

// GetRetryPolicy is a convenience function that returns the retry policy for a workflow task node name
func GetRetryPolicy(name string) *temporal.RetryPolicy {
	return GetContainer().GetRetryPolicy(name)
}

// IsWorkflowTask returns true if the node is a workflow task registered in this container
func (c *Container) IsWorkflowTask(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.workflowTasks[name]
	return exists
}

// IsWorkflowTask is a convenience function that returns true if the node is a workflow task
func IsWorkflowTask(name string) bool {
	return GetContainer().IsWorkflowTask(name)
}

// GetSchema returns the schema for a given workflow task node name
func (c *Container) GetSchema(name string) (*domain.NodeSchema, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	taskInfo, exists := c.workflowTasks[name]
	if !exists {
		return nil, false
	}
	return taskInfo.Schema, taskInfo.Schema != nil
}

// GetSchema is a convenience function that returns the schema for a workflow task node name
func GetSchema(name string) (*domain.NodeSchema, bool) {
	return GetContainer().GetSchema(name)
}
