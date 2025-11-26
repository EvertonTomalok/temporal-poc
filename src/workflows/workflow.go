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

var WorkflowName = "workflow"

func GenerateWorkflowID() string {
	return fmt.Sprintf("%s-%s", WorkflowName, uuid.New().String())
}

// StepConfig defines a single step in the dynamic workflow
type StepConfig struct {
	Node      string                 `json:"node"`                // The node name to execute
	GoTo      string                 `json:"go_to,omitempty"`     // Next step for simple linear flow (optional)
	Condition *domain.Condition      `json:"condition,omitempty"` // Conditional branching based on event types (optional)
	Schema    map[string]interface{} `json:"schema,omitempty"`    // Step schema data (validated against node schema)
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
func executeWorkflowNode(
	ctx workflow.Context,
	nodeName string,
	workflowID string,
	startTime time.Time,
	timeoutDuration time.Duration,
	schema map[string]interface{},
) (activities.NodeExecutionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing workflow task node", "node_name", nodeName)

	// Get the workflow node processor from register
	workflowNode, exists := register.GetWorkflowNode(nodeName)
	if !exists {
		return activities.NodeExecutionResult{}, fmt.Errorf("unknown workflow task node name: %s", nodeName)
	}

	// Get previous step results from memo to pass to workflow node
	// This allows workflow nodes to access metadata from previous steps
	previousResults := getPreviousResults(ctx)

	// Execute the workflow node directly (this waits for signals, handles timeouts, etc.)
	activityCtx := activities.ActivityContext{
		WorkflowID:      workflowID,
		NodeName:        nodeName, // Node name for timer identification in UI
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
		Schema:          schema,
		PreviousResults: previousResults, // Results/metadata from previous steps
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
func executeActivityNode(
	ctx workflow.Context,
	nodeName string,
	workflowID string,
	startTime time.Time,
	timeoutDuration time.Duration,
	schema map[string]interface{},
) (activities.NodeExecutionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing activity node", "node_name", nodeName)

	// Verify the activity exists in the register
	reg := register.GetInstance()
	nodeInfo, exists := reg.GetNodeInfo(nodeName)
	if !exists {
		return activities.NodeExecutionResult{
			EventType: domain.EventTypeConditionSatisfied,
		}, fmt.Errorf("unknown activity node name: %s", nodeName)
	}

	// Ensure this is actually an activity, not a workflow task
	if nodeInfo.Type != register.NodeTypeActivity {
		return activities.NodeExecutionResult{
			EventType: domain.EventTypeConditionSatisfied,
		}, fmt.Errorf("node '%s' is not an activity (it's a workflow task)", nodeName)
	}

	// Build activity context
	// Read ClientAnswered from search attributes
	sas := workflow.GetTypedSearchAttributes(ctx)
	clientAnswered, _ := sas.GetBool(core.ClientAnsweredField)

	// Get previous step results from memo to pass to activity
	// This allows activities to access metadata from previous steps
	previousResults := getPreviousResults(ctx)

	activityCtx := activities.ActivityContext{
		WorkflowID:      workflowID,
		NodeName:        nodeName, // Node name for identification in UI/logs
		ClientAnswered:  clientAnswered,
		StartTime:       startTime,
		TimeoutDuration: timeoutDuration,
		EventTime:       workflow.Now(ctx),
		EventType:       domain.EventTypeConditionSatisfied,
		Schema:          schema,
		PreviousResults: previousResults, // Results/metadata from previous steps
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
	var activityResult activities.ActivityResult
	err := workflow.ExecuteActivity(activityCtxWithOptions, nodeName, activityCtx).Get(ctx, &activityResult)
	if err != nil {
		logger.Error("Activity execution failed", "node_name", nodeName, "error", err)
		// Error returned from activity triggers retries - Temporal will handle retry logic
		return activities.NodeExecutionResult{
			Error:        err,
			ActivityName: nodeName,
			EventType:    domain.EventTypeConditionSatisfied,
		}, err
	}

	// Use event type from activity result, default to condition_satisfied if not set
	eventType := activityResult.EventType

	// Return success result with event type from activity
	result := activities.NodeExecutionResult{
		Error:        nil,
		ActivityName: nodeName,
		EventType:    eventType,
		Metadata:     activityResult.Metadata,
	}

	logger.Info("Activity node completed", "node_name", nodeName, "event_type", eventType)
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

// ValidateWorkflowConfig validates a WorkflowConfig and returns an error if invalid
// It also validates step inputs against node schemas
func ValidateWorkflowConfig(config WorkflowConfig) error {
	definition := convertToWorkflowDefinition(config)
	if err := validation.ValidateWorkflowDefinition(definition); err != nil {
		return err
	}

	// Validate step schemas against node schemas
	for stepName, stepConfig := range config.Steps {
		if stepConfig.Schema != nil {
			if err := validation.ValidateStepSchema(stepConfig.Node, stepConfig.Schema); err != nil {
				return fmt.Errorf("step '%s': %w", stepName, err)
			}
		}
	}

	return nil
}

// workflowExecutor holds the execution context for a workflow
type workflowExecutor struct {
	ctx             workflow.Context
	workflowID      string
	startTime       time.Time
	timeoutDuration time.Duration
	config          WorkflowConfig
}

// newWorkflowExecutor creates a new workflow executor
func newWorkflowExecutor(ctx workflow.Context, config WorkflowConfig) *workflowExecutor {
	workflowInfo := workflow.GetInfo(ctx)
	return &workflowExecutor{
		ctx:             ctx,
		workflowID:      workflowInfo.WorkflowExecution.ID,
		startTime:       workflow.Now(ctx),
		timeoutDuration: 24 * 30 * time.Hour,
		config:          config,
	}
}

// validateWorkflowDefinition validates the workflow configuration
func (we *workflowExecutor) validateWorkflowDefinition() error {
	definition := convertToWorkflowDefinition(we.config)
	if err := validation.ValidateWorkflowDefinition(definition); err != nil {
		workflow.GetLogger(we.ctx).Error("DynamicWorkflow: Workflow definition validation failed", "error", err)
		return err
	}
	return nil
}

// setFinalWorkflowStatus sets the final workflow status in the memo
func (we *workflowExecutor) setFinalWorkflowStatus(status string, err error) {
	finalMemo := map[string]interface{}{
		"workflow_status":       status,
		"workflow_completed_at": workflow.Now(we.ctx).UTC(),
	}
	if err != nil {
		finalMemo["workflow_error"] = err.Error()
	}
	if upsertErr := workflow.UpsertMemo(we.ctx, finalMemo); upsertErr != nil {
		workflow.GetLogger(we.ctx).Error("Failed to persist final workflow status memo", "error", upsertErr)
	}
}

// getPreviousResults retrieves the last activity result from memo to pass to next activities
// This allows activities to access metadata from previous steps
func getPreviousResults(ctx workflow.Context) map[string]interface{} {
	memo := workflow.GetInfo(ctx).Memo
	if memo == nil || len(memo.Fields) == 0 {
		return nil
	}

	// Look for last_activity_result key
	if lastResultPayload, exists := memo.Fields["last_activity_result"]; exists {
		var lastResult map[string]interface{}
		dataConverter := converter.GetDefaultDataConverter()
		if err := dataConverter.FromPayload(lastResultPayload, &lastResult); err != nil {
			workflow.GetLogger(ctx).Warn("Failed to decode last_activity_result from memo", "error", err)
			return nil
		}
		return lastResult
	}

	return nil
}

// setStepRunningStatus sets the running status memo for a step and returns the started_at time
func (we *workflowExecutor) setStepRunningStatus(stepName string, nodeName string) time.Time {
	startedAt := workflow.Now(we.ctx).UTC()
	stepResultKey := fmt.Sprintf("activity_result_%s", stepName)
	stepMemoRunning := map[string]interface{}{
		"step":       stepName,
		"node":       nodeName,
		"status":     "running",
		"started_at": startedAt,
	}
	memoRunning := map[string]interface{}{
		stepResultKey: stepMemoRunning,
	}
	if err := workflow.UpsertMemo(we.ctx, memoRunning); err != nil {
		workflow.GetLogger(we.ctx).Error("Failed to persist running status memo", "step", stepName, "error", err)
	}
	return startedAt
}

// executeStep executes a single workflow step
func (we *workflowExecutor) executeStep(stepName string, stepDef StepConfig) (activities.NodeExecutionResult, time.Time, error) {
	workflow.GetLogger(we.ctx).Info("Executing step", "step", stepName, "node", stepDef.Node)

	// Set status "running" when starting to process the node
	startedAt := we.setStepRunningStatus(stepName, stepDef.Node)

	// Execute the node - either as workflow task (waiter) or activity task
	var result activities.NodeExecutionResult
	var err error
	if isWorkflowTask(stepDef.Node) {
		result, err = executeWorkflowNode(we.ctx, stepDef.Node, we.workflowID, we.startTime, we.timeoutDuration, stepDef.Schema)
	} else {
		result, err = executeActivityNode(we.ctx, stepDef.Node, we.workflowID, we.startTime, we.timeoutDuration, stepDef.Schema)
	}
	return result, startedAt, err
}

// persistStepResult persists the step execution result in the memo
func (we *workflowExecutor) persistStepResult(stepName string, nodeName string, result activities.NodeExecutionResult, startedAt time.Time) {
	stepResultKey := fmt.Sprintf("activity_result_%s", stepName)
	stepMemo := map[string]interface{}{
		"step":         stepName,
		"node":         nodeName,
		"event_type":   string(result.EventType),
		"status":       "completed",
		"started_at":   startedAt, // Preserve started_at from when step began
		"completed_at": workflow.Now(we.ctx).UTC(),
	}
	if result.Error != nil {
		stepMemo["error"] = result.Error.Error()
		stepMemo["status"] = "failed"
	}
	if result.Metadata != nil {
		stepMemo["metadata"] = result.Metadata
	}

	memo := map[string]interface{}{
		stepResultKey:          stepMemo,
		"last_activity_result": stepMemo,
	}
	if err := workflow.UpsertMemo(we.ctx, memo); err != nil {
		workflow.GetLogger(we.ctx).Error("Failed to persist result memo", "step", stepName, "error", err)
	}
}

// determineNextStep determines the next step based on workflow definition (conditions or go_to)
func (we *workflowExecutor) determineNextStep(stepName string, stepDef StepConfig, result activities.NodeExecutionResult) string {
	// Check conditions first (conditional branching)
	if stepDef.Condition != nil && result.EventType != "" {
		nextStep := stepDef.Condition.GetNextStep(result.EventType)
		if nextStep != "" {
			workflow.GetLogger(we.ctx).Info("Conditional branch selected", "step", stepName, "event_type", result.EventType, "next_step", nextStep)
			return nextStep
		}
	}

	// If no condition matched, use go_to (linear flow)
	if stepDef.GoTo != "" {
		workflow.GetLogger(we.ctx).Info("Linear flow to next step", "step", stepName, "next_step", stepDef.GoTo)
		return stepDef.GoTo
	}

	// No next step defined
	return ""
}

// checkCircularReference checks if a step has been visited before (circular reference)
func (we *workflowExecutor) checkCircularReference(stepName string, visitedSteps map[string]bool) error {
	if visitedSteps[stepName] {
		return fmt.Errorf("circular workflow definition detected at step: %s", stepName)
	}
	visitedSteps[stepName] = true
	return nil
}

// getStepDefinition retrieves the step definition from the config
func (we *workflowExecutor) getStepDefinition(stepName string) (StepConfig, error) {
	stepDef, exists := we.config.Steps[stepName]
	if !exists {
		return StepConfig{}, fmt.Errorf("step definition not found: %s", stepName)
	}
	return stepDef, nil
}

// executeWorkflowConfig is a helper function that executes a workflow config
// This is shared between DynamicWorkflow and Workflow
func executeWorkflowConfig(ctx workflow.Context, config WorkflowConfig) error {
	executor := newWorkflowExecutor(ctx, config)
	workflow.GetLogger(ctx).Info("DynamicWorkflow started", "StartStep", config.StartStep, "StepsCount", len(config.Steps))

	// Validate workflow definition before starting execution
	if err := executor.validateWorkflowDefinition(); err != nil {
		executor.setFinalWorkflowStatus("failed", err)
		return err
	}

	// Execute workflow steps
	currentStep := config.StartStep
	visitedSteps := make(map[string]bool) // Track visited steps to prevent infinite loops

	for {
		// Check for infinite loops
		if err := executor.checkCircularReference(currentStep, visitedSteps); err != nil {
			workflow.GetLogger(executor.ctx).Error("Circular workflow definition detected", "step", currentStep)
			executor.setFinalWorkflowStatus("failed", err)
			return err
		}

		// Get step definition
		stepDef, err := executor.getStepDefinition(currentStep)
		if err != nil {
			workflow.GetLogger(executor.ctx).Error("Step definition not found", "step", currentStep)
			executor.setFinalWorkflowStatus("failed", err)
			return err
		}

		// Execute the step
		result, startedAt, err := executor.executeStep(currentStep, stepDef)
		if err != nil {
			workflow.GetLogger(executor.ctx).Error("Node execution failed", "step", currentStep, "node", stepDef.Node, "error", err)
			executor.setFinalWorkflowStatus("failed", err)
			return err
		}

		// Persist step result
		executor.persistStepResult(currentStep, stepDef.Node, result, startedAt)

		// Determine next step
		nextStep := executor.determineNextStep(currentStep, stepDef, result)

		// If no next step is defined, workflow ends
		if nextStep == "" {
			workflow.GetLogger(executor.ctx).Info("Workflow completed - no next step defined", "step", currentStep)
			executor.setFinalWorkflowStatus("completed", nil)
			return nil
		}

		currentStep = nextStep
	}
}
