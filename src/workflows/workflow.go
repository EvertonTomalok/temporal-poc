package src

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/core/domain"
	"temporal-poc/src/register"
	"temporal-poc/src/validation"
)

var AbandonedCartWorkflowName = "abandoned_cart"

func GenerateAbandonedCartWorkflowID() string {
	return fmt.Sprintf("%s-%s", AbandonedCartWorkflowName, uuid.New().String())
}

// Workflow is an agnostic workflow that delegates to nodes
// All business logic is handled by nodes in the /nodes directory
func Workflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("AbandonedCartWorkflow started")

	workflowInfo := workflow.GetInfo(ctx)
	workflowID := workflowInfo.WorkflowExecution.ID

	// Initialize workflow state
	startTime := workflow.Now(ctx)

	// Build workflow definition map
	// This defines the workflow structure with conditional branching
	// Future: This will be dynamically configured (e.g., from a drag-and-drop UI)
	definition := register.WorkflowDefinition{
		StartStep: "step_1",
		Steps: map[string]register.StepDefinition{
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

	// Validate workflow definition before starting execution
	if err := validation.ValidateWorkflowDefinition(definition); err != nil {
		logger.Error("AbandonedCartWorkflow: Workflow definition validation failed", "error", err)
		return err
	}

	// Create registry with workflow definition
	registry := register.NewActivityRegistryWithDefinition(definition)

	// Execute workflow nodes - registry orchestrates the flow using the map-based definition
	// Starts with step_1 (send_message), then follows the workflow definition
	// Conditional branching is handled based on event types returned by nodes (e.g., wait_answer returns "client-answered" or "timeout")
	if err := registry.Execute(ctx, workflowID, startTime, 24*30*time.Hour); err != nil {
		logger.Error("AbandonedCartWorkflow: Error executing workflow nodes", "error", err)
		return err
	}

	logger.Info("AbandonedCartWorkflow finishing")
	return nil
}
