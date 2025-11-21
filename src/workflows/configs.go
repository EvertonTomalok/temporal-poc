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
				Node: "send_message",
				GoTo: "step_2",
			},
			"step_2": {
				Node: "wait_answer",
				Condition: &domain.Condition{
					Satisfied: "step_3",
					Timeout:   "step_4",
				},
				Input: &domain.StepInput{
					TimeoutSeconds: 30,
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
