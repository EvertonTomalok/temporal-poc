package activities

import (
	"context"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	nodes "temporal-poc/src/nodes"
)

// ActivityContext holds the context passed to activities
type ActivityContext struct {
	WorkflowID      string
	NodeName        string // Node name for identification in UI/logs
	ClientAnswered  bool
	StartTime       time.Time
	TimeoutDuration time.Duration
	EventTime       time.Time
	EventType       domain.EventType
	Schema          map[string]interface{} // Step schema data (validated against node schema)
	PreviousResults map[string]interface{} // Results/metadata from previous steps (populated by workflow from memo)
}

// NodeExecutionResult contains information about the activity to execute
// NodeExecutionResult is agnostic and only knows about general results
// Note: ShouldContinue is deprecated - workflow definition (GoTo/Condition) controls flow
type NodeExecutionResult struct {
	Error error `json:"error,omitempty"`
	// Activity information - used by executor to call ExecuteActivity
	ActivityName string                 `json:"activity_name"`
	EventType    domain.EventType       `json:"event_type"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ActivityProcessor is a function type that processes an activity
// This is used by workflow nodes (processors) that run in workflow context
type ActivityProcessor func(ctx workflow.Context, activityCtx ActivityContext) NodeExecutionResult

// ActivityResult contains the result of an activity execution
// Activities can return an event type to control workflow flow
type ActivityResult struct {
	Metadata  map[string]interface{} // Metadata for future use (optional, can be nil)
	EventType domain.EventType       // Event type to control workflow flow (optional, defaults to condition_satisfied)
}

// ActivityFunction is the type signature for all activity functions
// This is used by actual Temporal activities that run in activity context
// Activities can return an ActivityResult with an event type to control workflow flow
// Return an error to trigger retries (retryable errors), or return nil with ActivityResult.Error set for non-retryable errors
type ActivityFunction func(ctx context.Context, activityCtx ActivityContext, deps core.Deps) (ActivityResult, error)

// ActivityInfo holds information about a registered activity
type ActivityInfo struct {
	Name        string
	Function    ActivityFunction
	RetryPolicy *temporal.RetryPolicy // Retry policy for the activity (nil means no retry)
	Schema      *domain.NodeSchema    // Input schema for the activity (optional)
	Visibility  string                // Visibility: "public" or "internal" (default: "public")
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

// RegisterActivity registers an activity function with a name and optional configuration
// This is called by each activity's init() function
// Options can be provided using WithRetryPolicy, WithSchema, WithPublicVisibility, WithInternalVisibility
func RegisterActivity(name string, fn ActivityFunction, opts ...func(*nodes.ActivityOptions)) {
	container := GetContainer()
	container.mu.Lock()
	defer container.mu.Unlock()

	// Apply default options
	options := &nodes.ActivityOptions{
		Visibility: "public", // Default to public
	}

	// Apply provided options
	for _, opt := range opts {
		opt(options)
	}

	container.activities[name] = ActivityInfo{
		Name:        name,
		Function:    fn,
		RetryPolicy: options.RetryPolicy,
		Schema:      options.Schema,
		Visibility:  options.Visibility,
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

// GetSchema returns the schema for a given activity name
func (c *Container) GetSchema(name string) (*domain.NodeSchema, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	activityInfo, exists := c.activities[name]
	if !exists {
		return nil, false
	}
	return activityInfo.Schema, activityInfo.Schema != nil
}

// GetSchema is a convenience function that returns the schema for an activity name
func GetSchema(name string) (*domain.NodeSchema, bool) {
	return GetContainer().GetSchema(name)
}

// GetVisibility returns the visibility for a given activity name
func (c *Container) GetVisibility(name string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	activityInfo, exists := c.activities[name]
	if !exists {
		return "public" // Default to public if not found
	}
	return activityInfo.Visibility
}

// GetVisibility is a convenience function that returns the visibility for an activity name
func GetVisibility(name string) string {
	return GetContainer().GetVisibility(name)
}
