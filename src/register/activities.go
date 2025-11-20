package register

import (
	"context"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
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

// Register is a singleton struct that holds registered activity processors
type Register struct {
	processors map[string]ActivityProcessor
	mu         sync.RWMutex
}

var (
	instance *Register
	once     sync.Once
)

// GetInstance returns the singleton instance of Register
func GetInstance() *Register {
	once.Do(func() {
		instance = &Register{
			processors: make(map[string]ActivityProcessor),
		}
	})
	return instance
}

// RegisterActivityProcessor registers an activity processor for a given node name
// This is called automatically via init() functions in node files
func (r *Register) RegisterActivityProcessor(nodeName string, processor ActivityProcessor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors[nodeName] = processor
}

// ProcessNodeActivity is a generic activity that processes a node
// The nodeName identifies which node to process (e.g., "client_answered_processor", "timeout_webhook")
// DEPRECATED: Use GetNamedActivityFunction instead to get activities with node names for UI display
func (r *Register) ProcessNodeActivity(ctx context.Context, nodeName string, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("ProcessNodeActivity: Processing node", "node_name", nodeName, "workflow_id", activityCtx.WorkflowID)

	r.mu.RLock()
	processor, exists := r.processors[nodeName]
	r.mu.RUnlock()

	if !exists {
		logger.Error("Unknown node name", "node_name", nodeName)
		return nil // Don't fail workflow for unknown nodes
	}

	return processor(ctx, activityCtx)
}

// GetNamedActivityFunction returns an activity function for a specific node name
// This allows the activity to appear with the node name in the Temporal UI
// The nodeName is captured in the closure to ensure the correct processor is called
func (r *Register) GetNamedActivityFunction(nodeName string) func(context.Context, ActivityContext) error {
	// Capture nodeName in the closure to avoid closure variable issues
	capturedNodeName := nodeName
	return func(ctx context.Context, activityCtx ActivityContext) error {
		logger := activity.GetLogger(ctx)
		logger.Info("Processing node", "node_name", capturedNodeName, "workflow_id", activityCtx.WorkflowID)

		r.mu.RLock()
		processor, exists := r.processors[capturedNodeName]
		r.mu.RUnlock()

		if !exists {
			logger.Error("Unknown node name", "node_name", capturedNodeName)
			return nil // Don't fail workflow for unknown nodes
		}

		return processor(ctx, activityCtx)
	}
}

// AllProcessorsNames returns a slice of strings containing all registered processor names
func (r *Register) AllProcessorsNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodeNames := make([]string, 0, len(r.processors))
	for nodeName := range r.processors {
		nodeNames = append(nodeNames, nodeName)
	}
	return nodeNames
}

// Legacy functions for backward compatibility
// These delegate to the singleton instance

// RegisterActivityProcessor registers an activity processor for a given node name
// This is called automatically via init() functions in node files
func RegisterActivityProcessor(nodeName string, processor ActivityProcessor) {
	GetInstance().RegisterActivityProcessor(nodeName, processor)
}

// ProcessNodeActivity is a generic activity that processes a node
// The nodeName identifies which node to process (e.g., "client_answered_processor", "timeout_webhook")
// DEPRECATED: Use GetNamedActivityFunction instead to get activities with node names for UI display
func ProcessNodeActivity(ctx context.Context, nodeName string, activityCtx ActivityContext) error {
	return GetInstance().ProcessNodeActivity(ctx, nodeName, activityCtx)
}

// GetNamedActivityFunction returns an activity function for a specific node name
// This allows the activity to appear with the node name in the Temporal UI
// The nodeName is captured in the closure to ensure the correct processor is called
func GetNamedActivityFunction(nodeName string) func(context.Context, ActivityContext) error {
	return GetInstance().GetNamedActivityFunction(nodeName)
}

// GetAllRegisteredNodeNames returns all registered node names
// This is used to register all named activities in the worker
func GetAllRegisteredNodeNames() []string {
	return GetInstance().AllProcessorsNames()
}
