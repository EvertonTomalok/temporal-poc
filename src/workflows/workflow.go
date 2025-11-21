package workflows

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/nodes/activities"
	"temporal-poc/src/register"
	"temporal-poc/src/validation"
)

var AbandonedCartWorkflowName = "abandoned_cart"

func GenerateAbandonedCartWorkflowID() string {
	return fmt.Sprintf("%s-%s", AbandonedCartWorkflowName, uuid.New().String())
}

// StepConfig defines a single step in the dynamic workflow
type StepConfig struct {
	Node      string            `json:"node"`                // The node name to execute
	GoTo      string            `json:"go_to,omitempty"`     // Next step for simple linear flow (optional)
	Condition *domain.Condition `json:"condition,omitempty"` // Conditional branching based on event types (optional)
}

// WorkflowConfig defines the configuration for the dynamic workflow
type WorkflowConfig struct {
	StartStep string                `json:"start_step"` // The starting step name
	Steps     map[string]StepConfig `json:"steps"`      // Map of step names to step definitions
}

// WorkflowExecutionConfig contains workflow options and configuration
type WorkflowExecutionConfig struct {
	Options      client.StartWorkflowOptions
	Config       WorkflowConfig
	WorkflowName string
}

// isWorkflowTask determines if a node is a workflow task (waiter) or an activity task
// Workflow tasks use workflow.Sleep, timers, or signals and should execute directly in workflow
// Activity tasks should execute via workflow.ExecuteActivity
func isWorkflowTask(nodeName string) bool {
	return register.IsWorkflowTask(nodeName)
}

// BuildDefaultWorkflowDefinition builds the default workflow definition
// This can be called from the server to build the workflow config dynamically
func BuildDefaultWorkflowDefinition() WorkflowConfig {
	return WorkflowConfig{
		StartStep: "step_1",
		Steps: map[string]StepConfig{
			"step_1": {
				Node: "send_message",
				GoTo: "step_2",
			},
			"step_2": {
				Node: "wait_answer",
				Condition: &domain.Condition{
					Satisfied: "step_3",
					Timeout:   "step_4",
				},
			},
			"step_3": {
				Node: "notify_creator",
			},
			"step_4": {
				Node: "webhook",
				GoTo: "step_5",
			},
			"step_5": {
				Node: "explicity_wait",
				GoTo: "step_6",
			},
			"step_6": {
				Node: "send_message",
			},
		},
	}
}

// DynamicWorkflow executes workflow steps dynamically based on configuration
// Waiters (wait_answer, explicity_wait) execute directly in workflow
// Activities (send_message, notify_creator, webhook) execute via workflow.ExecuteActivity
func DynamicWorkflow(ctx workflow.Context, args converter.EncodedValues) error {
	var config WorkflowConfig
	if err := args.Get(&config); err != nil {
		return fmt.Errorf("failed to decode workflow config: %w", err)
	}

	return executeWorkflowConfig(ctx, config)
}

// executeWorkflowNode executes a waiter node directly in the workflow (no activity call)
func executeWorkflowNode(ctx workflow.Context, nodeName string, workflowID string, startTime time.Time, timeoutDuration time.Duration) (activities.NodeExecutionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing workflow task node", "node_name", nodeName)

	// Get the workflow node processor from register
	workflowNode, exists := register.GetWorkflowNode(nodeName)
	if !exists {
		return activities.NodeExecutionResult{}, fmt.Errorf("unknown workflow task node name: %s", nodeName)
	}

	// Execute the workflow node directly (this waits for signals, handles timeouts, etc.)
	activityCtx := activities.ActivityContext{
		WorkflowID:      workflowID,
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
	}

	result := workflowNode(ctx, activityCtx)
	if result.Error != nil {
		logger.Error("Workflow task node execution failed", "node_name", nodeName, "error", result.Error)
		return result, result.Error
	}

	logger.Info("Workflow task node completed", "node_name", nodeName)
	return result, nil
}

// executeActivityNode executes an activity node via workflow.ExecuteActivity
// Activities are called directly as Temporal activities - they don't have processors in workflow context
func executeActivityNode(ctx workflow.Context, nodeName string, workflowID string, startTime time.Time, timeoutDuration time.Duration) (activities.NodeExecutionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing activity node", "node_name", nodeName)

	// Verify the activity exists in the register
	reg := register.GetInstance()
	nodeInfo, exists := reg.GetNodeInfo(nodeName)
	if !exists {
		return activities.NodeExecutionResult{}, fmt.Errorf("unknown activity node name: %s", nodeName)
	}

	// Ensure this is actually an activity, not a workflow task
	if nodeInfo.Type != register.NodeTypeActivity {
		return activities.NodeExecutionResult{}, fmt.Errorf("node '%s' is not an activity (it's a workflow task)", nodeName)
	}

	// Build activity context
	// Read ClientAnswered from search attributes
	sas := workflow.GetTypedSearchAttributes(ctx)
	clientAnswered, _ := sas.GetBool(core.ClientAnsweredField)

	activityCtx := activities.ActivityContext{
		WorkflowID:      workflowID,
		ClientAnswered:  clientAnswered,
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
		EventTime:       workflow.Now(ctx),
		EventType:       domain.EventTypeConditionSatisfied,
	}

	// Set activity timeout - activities should have reasonable timeouts (max 10 minutes)
	activityTimeout := 5 * time.Minute // Default to 5 minutes for activities
	if timeoutDuration > 0 && timeoutDuration < 10*time.Minute {
		activityTimeout = timeoutDuration
	}

	// Get retry policy directly from activities container to ensure we use the activity's own retry policy
	// This guarantees that each activity uses its registered retry policy
	retryPolicy := activities.GetRetryPolicy(nodeName)
	if retryPolicy == nil {
		// If no retry policy is registered, use a default one
		logger.Warn("No retry policy registered for activity, using default", "node_name", nodeName)
		retryPolicy = &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 1.1,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    15,
		}
	} else {
		logger.Info("Using registered retry policy for activity", "node_name", nodeName,
			"initial_interval", retryPolicy.InitialInterval,
			"backoff_coefficient", retryPolicy.BackoffCoefficient,
			"maximum_interval", retryPolicy.MaximumInterval,
			"maximum_attempts", retryPolicy.MaximumAttempts)
	}

	// Set activity options with timeout and retry policy before executing
	// HeartbeatTimeout is required for heartbeats to be tracked and visible in UI
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: activityTimeout,
		HeartbeatTimeout:    60 * time.Second,
		RetryPolicy:         retryPolicy,
	}
	activityCtxWithOptions := workflow.WithActivityOptions(ctx, ao)

	// Execute activity using node name as the activity name
	// The activity function is registered in the worker and will be called by Temporal
	var activityResult error
	err := workflow.ExecuteActivity(activityCtxWithOptions, nodeName, activityCtx).Get(ctx, &activityResult)
	if err != nil {
		logger.Error("Activity execution failed", "node_name", nodeName, "error", err)
		return activities.NodeExecutionResult{
			Error:        err,
			ActivityName: nodeName,
			EventType:    domain.EventTypeConditionSatisfied,
		}, err
	}

	// Return success result
	result := activities.NodeExecutionResult{
		Error:        nil,
		ActivityName: nodeName,
		EventType:    domain.EventTypeConditionSatisfied,
	}

	logger.Info("Activity node completed", "node_name", nodeName)
	return result, nil
}

// convertToWorkflowDefinition converts WorkflowConfig to register.WorkflowDefinition for validation
func convertToWorkflowDefinition(config WorkflowConfig) register.WorkflowDefinition {
	steps := make(map[string]register.StepDefinition)
	for stepName, stepConfig := range config.Steps {
		steps[stepName] = register.StepDefinition{
			Node:      stepConfig.Node,
			GoTo:      stepConfig.GoTo,
			Condition: stepConfig.Condition,
		}
	}
	return register.WorkflowDefinition{
		StartStep: config.StartStep,
		Steps:     steps,
	}
}

// executeWorkflowConfig is a helper function that executes a workflow config
// This is shared between DynamicWorkflow and Workflow
func executeWorkflowConfig(ctx workflow.Context, config WorkflowConfig) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("DynamicWorkflow started", "StartStep", config.StartStep, "StepsCount", len(config.Steps))

	workflowInfo := workflow.GetInfo(ctx)
	workflowID := workflowInfo.WorkflowExecution.ID
	startTime := workflow.Now(ctx)
	timeoutDuration := 24 * 30 * time.Hour

	// Convert WorkflowConfig to register.WorkflowDefinition for validation
	definition := convertToWorkflowDefinition(config)

	// Validate workflow definition before starting execution
	if err := validation.ValidateWorkflowDefinition(definition); err != nil {
		logger.Error("DynamicWorkflow: Workflow definition validation failed", "error", err)
		return err
	}

	// Execute workflow steps
	currentStep := config.StartStep
	visitedSteps := make(map[string]bool) // Track visited steps to prevent infinite loops

	for {
		// Check for infinite loops
		if visitedSteps[currentStep] {
			logger.Error("Circular workflow definition detected", "step", currentStep)
			return fmt.Errorf("circular workflow definition detected at step: %s", currentStep)
		}
		visitedSteps[currentStep] = true

		// Get step definition
		stepDef, exists := config.Steps[currentStep]
		if !exists {
			logger.Error("Step definition not found", "step", currentStep)
			return fmt.Errorf("step definition not found: %s", currentStep)
		}

		logger.Info("Executing step", "step", currentStep, "node", stepDef.Node)

		// Execute the node - either as workflow task (waiter) or activity task
		var result activities.NodeExecutionResult
		var err error

		if isWorkflowTask(stepDef.Node) {
			// Execute workflow task directly in workflow (no activity call)
			result, err = executeWorkflowNode(ctx, stepDef.Node, workflowID, startTime, timeoutDuration)
		} else {
			// Execute activity task via workflow.ExecuteActivity
			result, err = executeActivityNode(ctx, stepDef.Node, workflowID, startTime, timeoutDuration)
		}

		if err != nil {
			logger.Error("Node execution failed", "step", currentStep, "node", stepDef.Node, "error", err)
			return err
		}

		// Persist result memo for the step in the workflow
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
		}

		// Determine next step based on workflow definition (conditions or go_to)
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
