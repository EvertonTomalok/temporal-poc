package workflows

import (
	"temporal-poc/src/core/domain"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// BuildDefaultWorkflowDefinition builds the default workflow definition
// This can be called from the server to build the workflow config dynamically
func BuildDefaultWorkflowDefinition() WorkflowConfig {
	return WorkflowConfig{
		StartStep: "step_1",
		Steps: map[string]StepConfig{
			"step_1": {
				Node: "bought_any_offer", // Activity task with conditional branching
				Condition: &domain.Condition{
					Satisfied:    "step_2", // If offer found (condition_satisfied)
					NotSatisfied: "step_3", // If no offer found (condition_not_satisfied)
				},
				Schema: map[string]interface{}{ // Schema input validated against node schema
					"last_minutes": int64(60),
				},
			},
			"step_2": {
				Node: "notify_creator",
			},
			"step_3": {
				Node: "send_message", // Activity task
				GoTo: "step_4",       // Linear flow
			},
			"step_4": {
				Node: "wait_answer", // Workflow task (waiter)
				Condition: &domain.Condition{
					Satisfied: "step_2", // If signal received
					Timeout:   "step_5", // If timeout occurs
				},
				Schema: map[string]interface{}{ // Schema input validated against node schema
					"timeout_seconds": int64(30),
				},
			},
			"step_5": {
				Node: "webhook", // Activity task
				GoTo: "step_6",
			},
			"step_6": {
				Node: "explicity_wait", // Workflow task (waiter)
				GoTo: "step_7",
				Schema: map[string]interface{}{ // Schema input validated against node schema
					"wait_seconds": int64(15),
				},
			},
			"step_7": {
				Node: "send_message", // Activity task
				// Workflow ends here
			},
		},
	}
}

// BuildDefaultWorkflowExecutionConfig builds the default workflow execution configuration
// This includes both the workflow options and the workflow config
func BuildDefaultWorkflowExecutionConfig(workflowID string) WorkflowExecutionConfig {
	config := BuildDefaultWorkflowDefinition()
	return WorkflowExecutionConfig{
		Options: client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: domain.PrimaryWorkflowTaskQueue,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1, // No retries
			},
		},
		Config:       config,
		WorkflowName: "DynamicWorkflow",
	}
}
