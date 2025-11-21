package activities

import (
	"context"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core/domain"
)

// ActivityContext holds the context passed to activities
type ActivityContext struct {
	WorkflowID      string
	ClientAnswered  bool
	StartTime       time.Time
	TimeoutDuration time.Duration
	EventTime       time.Time
	EventType       domain.EventType
	Input           *domain.StepInput // Step input (optional) - can be *domain.StepInput or other types
}

// NodeExecutionResult contains information about the activity to execute
// NodeExecutionResult is agnostic and only knows about general results
// Note: ShouldContinue is deprecated - workflow definition (GoTo/Condition) controls flow
type NodeExecutionResult struct {
	Error error
	// Activity information - used by executor to call ExecuteActivity
	ActivityName string
	EventType    domain.EventType
}

// ActivityProcessor is a function type that processes an activity
// This is used by workflow nodes (processors) that run in workflow context
type ActivityProcessor func(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult

// ActivityFunction is the type signature for all activity functions
// This is used by actual Temporal activities that run in activity context
type ActivityFunction func(ctx context.Context, activityCtx ActivityContext) error

// ActivityInfo holds information about a registered activity
type ActivityInfo struct {
	Name        string
	Function    ActivityFunction
	RetryPolicy *temporal.RetryPolicy // Retry policy for the activity (nil means no retry)
}

// Container holds all registered activities
type Container struct {
	activities map[string]ActivityInfo
	mu         sync.RWMutex
}

var (
	containerInstance *Container
	containerOnce     sync.Once
)

// GetContainer returns the singleton instance of Container
func GetContainer() *Container {
	containerOnce.Do(func() {
		containerInstance = &Container{
			activities: make(map[string]ActivityInfo),
		}
	})
	return containerInstance
}

// RegisterActivity registers an activity function with a name and optional retry policy
// This is called by each activity's init() function
// If retryPolicy is nil, no retry policy will be applied (empty retry policy)
func RegisterActivity(name string, fn ActivityFunction, retryPolicy *temporal.RetryPolicy) {
	container := GetContainer()
	container.mu.Lock()
	defer container.mu.Unlock()
	container.activities[name] = ActivityInfo{
		Name:        name,
		Function:    fn,
		RetryPolicy: retryPolicy,
	}
}

// GetActivity returns the activity function for a given name
func (c *Container) GetActivity(name string) (ActivityFunction, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	activityInfo, exists := c.activities[name]
	if !exists {
		return nil, false
	}
	return activityInfo.Function, true
}

// GetAllActivityNames returns all registered activity names
func (c *Container) GetAllActivityNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.activities))
	for name := range c.activities {
		names = append(names, name)
	}
	return names
}

// HasActivity returns true if an activity with the given name is registered
func (c *Container) HasActivity(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.activities[name]
	return exists
}

// GetRetryPolicy returns the retry policy for a given activity name
// If no retry policy is configured, return a default retry policy
func (c *Container) GetRetryPolicy(name string) *temporal.RetryPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	activityInfo, exists := c.activities[name]
	if !exists {
		return &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 1.1,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    15,
		}
	}
	return activityInfo.RetryPolicy
}

// Convenience functions that use the singleton instance

// GetActivity is a convenience function that returns the activity function for a name
func GetActivity(name string) (ActivityFunction, bool) {
	return GetContainer().GetActivity(name)
}

// GetAllActivityNames is a convenience function that returns all registered activity names
func GetAllActivityNames() []string {
	return GetContainer().GetAllActivityNames()
}

// HasActivity is a convenience function that returns true if an activity is registered
func HasActivity(name string) bool {
	return GetContainer().HasActivity(name)
}

// GetRetryPolicy is a convenience function that returns the retry policy for an activity name
func GetRetryPolicy(name string) *temporal.RetryPolicy {
	return GetContainer().GetRetryPolicy(name)
}
