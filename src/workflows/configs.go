package workflows

import (
	"temporal-poc/src/core/domain"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

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
