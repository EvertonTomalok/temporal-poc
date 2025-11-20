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

// activityRegistry is a container that holds registered activity processors
type activityRegistry struct {
	processors map[string]ActivityProcessor
	mu         sync.RWMutex
}

var registry = &activityRegistry{
	processors: make(map[string]ActivityProcessor),
}

// RegisterActivityProcessor registers an activity processor for a given node name
// This is called automatically via init() functions in node files
func RegisterActivityProcessor(nodeName string, processor ActivityProcessor) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.processors[nodeName] = processor
}

// ProcessNodeActivity is a generic activity that processes a node
// The nodeName identifies which node to process (e.g., "client_answered_processor", "timeout_webhook")
func ProcessNodeActivity(ctx context.Context, nodeName string, activityCtx ActivityContext) error {
	logger := activity.GetLogger(ctx)
	logger.Info("ProcessNodeActivity: Processing node", "node_name", nodeName, "workflow_id", activityCtx.WorkflowID)

	registry.mu.RLock()
	processor, exists := registry.processors[nodeName]
	registry.mu.RUnlock()

	if !exists {
		logger.Error("Unknown node name", "node_name", nodeName)
		return nil // Don't fail workflow for unknown nodes
	}

	return processor(ctx, activityCtx)
}
