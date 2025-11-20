package nodes

import (
	"context"
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"
)

// ActivityContext holds the context passed to activities
type ActivityContext struct {
	WorkflowID      string
	ClientAnswered  bool
	StartTime       time.Time
	TimeoutDuration time.Duration
	EventTime       time.Time
	EventType       string // "client-answered" or "timeout"
}

// ActivityProcessor is a function type that processes an activity
type ActivityProcessor func(ctx context.Context, activityCtx ActivityContext) error

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

// ActivityRegistry maintains the order of nodes to be executed dynamically
type ActivityRegistry struct {
	NodeNames []string // Ordered list of node names to execute
}

// WorkflowNode is a function type that represents a workflow node
// It returns whether to continue to the next node and any error
type WorkflowNode func(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) NodeExecutionResult

// NodeInfo holds information about a registered node
type NodeInfo struct {
	Name         string
	Processor    ActivityProcessor
	WorkflowNode WorkflowNode
}

// Container holds all registered nodes with their processors and workflow nodes
type Container struct {
	nodes map[string]NodeInfo
	mu    sync.RWMutex
}

var (
	containerInstance *Container
	containerOnce     sync.Once
)

// GetContainer returns the singleton instance of Container
func GetContainer() *Container {
	containerOnce.Do(func() {
		containerInstance = &Container{
			nodes: make(map[string]NodeInfo),
		}
	})
	return containerInstance
}

// RegisterNode registers a node name, processor, and workflow node in the container
// This is called by each node's init() function
func RegisterNode(name string, processor ActivityProcessor, workflowNode WorkflowNode) {
	container := GetContainer()
	container.mu.Lock()
	defer container.mu.Unlock()
	container.nodes[name] = NodeInfo{
		Name:         name,
		Processor:    processor,
		WorkflowNode: workflowNode,
	}
}

// GetProcessor returns the processor for a given node name
func (c *Container) GetProcessor(name string) (ActivityProcessor, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	nodeInfo, exists := c.nodes[name]
	if !exists {
		return nil, false
	}
	return nodeInfo.Processor, true
}

// GetAllNodeNames returns all registered node names
func (c *Container) GetAllNodeNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodeNames := make([]string, 0, len(c.nodes))
	for name := range c.nodes {
		nodeNames = append(nodeNames, name)
	}
	return nodeNames
}

// GetAllNodeNames is a convenience function that returns all registered node names
// This is the main entry point for getting all node names
func GetAllNodeNames() []string {
	return GetContainer().GetAllNodeNames()
}

// GetProcessor is a convenience function that returns the processor for a node name
func GetProcessor(name string) (ActivityProcessor, bool) {
	return GetContainer().GetProcessor(name)
}

// GetWorkflowNode returns the workflow node for a given node name
func (c *Container) GetWorkflowNode(name string) (WorkflowNode, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	nodeInfo, exists := c.nodes[name]
	if !exists {
		return nil, false
	}
	return nodeInfo.WorkflowNode, true
}

// GetWorkflowNode is a convenience function that returns the workflow node for a node name
func GetWorkflowNode(name string) (WorkflowNode, bool) {
	return GetContainer().GetWorkflowNode(name)
}
