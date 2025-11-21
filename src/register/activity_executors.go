package register

import (
	"fmt"
	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/nodes"
	"time"

	"go.temporal.io/sdk/workflow"
)

// StepDefinition defines a single step in the workflow
// It can have either a simple "go_to" for linear flow or "conditions" for conditional branching
type StepDefinition struct {
	Node      string            `json:"node"`      // The node name to execute
	GoTo      string            `json:"go_to"`     // Next step for simple linear flow (optional)
	Condition *domain.Condition `json:"condition"` // Conditional branching based on event types (optional)
}

// WorkflowDefinition defines the entire workflow structure with steps and a starter step
type WorkflowDefinition struct {
	Steps     map[string]StepDefinition `json:"steps"`      // Map of step names to step definitions
	StartStep string                    `json:"start_step"` // The starting step name
}

// ExecuteProcessNodeActivity executes the activity for a specific node
// Uses the node name as the activity name so it appears correctly in the Temporal UI
// Activities don't have timers - timers are only in workflow nodes
// timeoutDuration is used to set the activity timeout - nodes handle their own timeout logic
func ExecuteProcessNodeActivity(ctx workflow.Context, nodeName string, activityCtx ActivityContext, timeoutDuration time.Duration) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExecuteProcessNodeActivity: Executing named activity", "node_name", nodeName)

	// Set activity timeout - activities should have reasonable timeouts (max 10 minutes)
	// Use a reasonable default if timeoutDuration is too large or invalid
	activityTimeout := 5 * time.Minute // Default to 5 minutes for activities
	if timeoutDuration > 0 {
		// Use the provided timeout if it's reasonable (less than 10 minutes)
		activityTimeout = timeoutDuration
	}

	// Set activity options with timeout before executing
	// Activities require StartToCloseTimeout to be set
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: activityTimeout,
	}
	activityCtxWithOptions := workflow.WithActivityOptions(ctx, ao)

	// Execute activity using node name as the activity name (appears in UI)
	// The activity must be registered with this exact name in the worker
	// Using string name ensures it appears correctly in Temporal UI
	// Use activityCtxWithOptions for ExecuteActivity to ensure timeout is applied
	var result error
	err := workflow.ExecuteActivity(activityCtxWithOptions, nodeName, activityCtx).Get(ctx, &result)
	if err != nil {
		logger.Error("ExecuteProcessNodeActivity: Activity execution failed", "node_name", nodeName, "error", err)
		return err
	}
	logger.Info("ExecuteProcessNodeActivity: Activity completed", "node_name", nodeName)
	return result
}

// NodeExecutionResult is an alias for nodes.NodeExecutionResult
type NodeExecutionResult = nodes.NodeExecutionResult

// ActivityRegistry holds the workflow definition and execution state
type ActivityRegistry struct {
	Definition WorkflowDefinition // Workflow definition with steps and start step
}

// NewActivityRegistryWithDefinition creates a new activity registry with a workflow definition
// The definition contains steps map and start step, allowing for conditional branching
func NewActivityRegistryWithDefinition(definition WorkflowDefinition) *ActivityRegistry {
	return &ActivityRegistry{
		Definition: definition,
	}
}

// Execute orchestrates workflow nodes using the map-based definition
// It handles conditional branching based on event types returned by nodes
// All node execution goes through ExecuteActivity - this is the only entry point
func (r *ActivityRegistry) Execute(ctx workflow.Context, workflowID string, startTime time.Time, timeoutDuration time.Duration) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow node orchestration (map-based)", "start_step", r.Definition.StartStep)

	currentStep := r.Definition.StartStep
	visitedSteps := make(map[string]bool) // Track visited steps to prevent infinite loops

	for {
		// Check for infinite loops
		if visitedSteps[currentStep] {
			logger.Error("Circular workflow definition detected", "step", currentStep)
			return fmt.Errorf("circular workflow definition detected at step: %s", currentStep)
		}
		visitedSteps[currentStep] = true

		// Get step definition
		stepDef, exists := r.Definition.Steps[currentStep]
		if !exists {
			logger.Error("Step definition not found", "step", currentStep)
			return fmt.Errorf("step definition not found: %s", currentStep)
		}

		logger.Info("Executing step", "step", currentStep, "node", stepDef.Node)

		// Execute the node for this step
		result, err := ExecuteActivity(ctx, stepDef.Node, workflowID, startTime, timeoutDuration, r)
		if err != nil {
			logger.Error("Node execution failed", "step", currentStep, "node", stepDef.Node, "error", err)
			return err
		}

		// Persist result memo for the activity in the workflow
		// Use step-specific key to preserve results for all steps
		stepResultKey := fmt.Sprintf("activity_result_%s", currentStep)
		memo := map[string]interface{}{
			stepResultKey: map[string]interface{}{
				"step":          currentStep,
				"node":          stepDef.Node,
				"activity_name": result.ActivityName,
				"event_type":    string(result.EventType),
				"completed_at":  workflow.Now(ctx).UTC(),
			},
			"last_activity_result": map[string]interface{}{
				"step":          currentStep,
				"node":          stepDef.Node,
				"activity_name": result.ActivityName,
				"event_type":    string(result.EventType),
				"completed_at":  workflow.Now(ctx).UTC(),
			},
		}
		if err := workflow.UpsertMemo(ctx, memo); err != nil {
			logger.Error("Failed to persist result memo", "step", currentStep, "error", err)
		} else {
			logger.Info("Result memo persisted", "step", currentStep, "node", stepDef.Node)
		}

		// Determine next step based on workflow definition (conditions or go_to)
		// The workflow definition controls flow, not individual nodes
		nextStep := ""

		// Check conditions first (conditional branching)
		if stepDef.Condition != nil && result.EventType != "" {
			nextStep = stepDef.Condition.GetNextStep(result.EventType)
			if nextStep != "" {
				logger.Info("Conditional branch selected", "step", currentStep, "event_type", result.EventType, "next_step", nextStep)
			}
		}

		// If no condition matched, use go_to (linear flow)
		if nextStep == "" && stepDef.GoTo != "" {
			nextStep = stepDef.GoTo
			logger.Info("Linear flow to next step", "step", currentStep, "next_step", nextStep)
		}

		// If no next step is defined, workflow ends
		if nextStep == "" {
			logger.Info("Workflow completed - no next step defined", "step", currentStep)
			return nil
		}

		currentStep = nextStep
	}
}

// ExecuteActivity is the ONLY entry point for executing nodes
// It first executes the workflow node (for signal waiting, etc.), then ExecuteProcessNodeActivity for the activity
// ExecuteProcessNodeActivity is the only function allowed to run activity processors
func ExecuteActivity(ctx workflow.Context, nodeName string, workflowID string, startTime time.Time, timeoutDuration time.Duration, registry *ActivityRegistry) (NodeExecutionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExecuteActivity: Executing node", "node_name", nodeName)

	// Get the workflow node from nodes (workflow nodes handle signal waiting, timeouts, etc.)
	workflowNode, exists := nodes.GetWorkflowNode(nodeName)

	if !exists {
		logger.Error("Unknown workflow node name", "node_name", nodeName)
		return NodeExecutionResult{
			Error: nil,
		}, nil
	}

	// Execute the workflow node first (this waits for signals, handles timeouts, etc.)
	logger.Info("ExecuteActivity: Executing workflow node", "node_name", nodeName)
	result := workflowNode(ctx, ActivityContext{
		WorkflowID:      workflowID,
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
	})
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
	// Read ClientAnswered from search attributes (agnostic - NodeExecutionResult doesn't know about it)
	sas := workflow.GetTypedSearchAttributes(ctx)
	clientAnswered, _ := sas.GetBool(core.ClientAnsweredField)

	activityCtx := ActivityContext{
		WorkflowID:      workflowID,
		ClientAnswered:  clientAnswered,
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
		EventTime:       workflow.Now(ctx),
		EventType:       result.EventType,
	}

	// Execute the activity processor - ExecuteProcessNodeActivity is the only function allowed to run activity processors
	// Activities don't have timers - timers are only in workflow nodes
	// Pass timeoutDuration to the activity executor - nodes handle their own timeout logic
	logger.Info("ExecuteActivity: Calling ExecuteProcessNodeActivity", "node_name", nodeName, "activity_name", activityName)
	if err := ExecuteProcessNodeActivity(ctx, activityName, activityCtx, timeoutDuration); err != nil {
		logger.Error("Activity execution failed", "node_name", nodeName, "activity_name", activityName, "error", err)
		return result, err
	}

	logger.Info("ExecuteActivity: Node completed", "node_name", nodeName)
	return result, nil
}
