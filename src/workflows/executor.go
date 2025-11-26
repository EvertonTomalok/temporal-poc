package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/nodes/activities"
	"temporal-poc/src/validation"
)

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
