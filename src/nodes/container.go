package nodes

import (
	"sync"
	"temporal-poc/src/core/domain"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ActivityContext holds the context passed to activities
type ActivityContext struct {
	WorkflowID      string
	ClientAnswered  bool
	StartTime       time.Time
	TimeoutDuration time.Duration
	EventTime       time.Time
	EventType       domain.EventType
}

// ActivityProcessor is a function type that processes an activity
type ActivityProcessor func(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult

// NodeExecutionResult contains information about the activity to execute
// NodeExecutionResult is agnostic and only knows about general results
// Note: ShouldContinue is deprecated - workflow definition (GoTo/Condition) controls flow
type NodeExecutionResult struct {
	Error error
	// Activity information - used by executor to call ExecuteActivity
	ActivityName string
	EventType    domain.EventType
}

// ActivityRegistry maintains the order of nodes to be executed dynamically
type ActivityRegistry struct {
	NodeNames []string // Ordered list of node names to execute
}

// NodeInfo holds information about a registered node
type NodeInfo struct {
	Name        string
	Processor   ActivityProcessor
	RetryPolicy *temporal.RetryPolicy // Retry policy for the node's activity (nil means no retry)
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

// RegisterNode registers a node name and processor in the container
// This is called by each node's init() function
// If retryPolicy is nil, no retry policy will be applied (empty retry policy)
func RegisterNode(name string, processor ActivityProcessor, retryPolicy *temporal.RetryPolicy) {
	container := GetContainer()
	container.mu.Lock()
	defer container.mu.Unlock()
	container.nodes[name] = NodeInfo{
		Name:        name,
		Processor:   processor,
		RetryPolicy: retryPolicy,
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
func (c *Container) GetWorkflowNode(name string) (ActivityProcessor, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	nodeInfo, exists := c.nodes[name]
	if !exists {
		return nil, false
	}
	return nodeInfo.Processor, true
}

// GetWorkflowNode is a convenience function that returns the workflow node for a node name
func GetWorkflowNode(name string) (ActivityProcessor, bool) {
	return GetContainer().GetWorkflowNode(name)
}

// GetRetryPolicy returns the retry policy for a given node name
// If no retry policy is configured, return a simple retry policy with maximum attempts set to 1
func (c *Container) GetRetryPolicy(name string) *temporal.RetryPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	nodeInfo, exists := c.nodes[name]
	if !exists {
		return &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 1.1,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    15,
		}
	}
	return nodeInfo.RetryPolicy
}

// GetRetryPolicy is a convenience function that returns the retry policy for a node name
func GetRetryPolicy(name string) *temporal.RetryPolicy {
	return GetContainer().GetRetryPolicy(name)
}
