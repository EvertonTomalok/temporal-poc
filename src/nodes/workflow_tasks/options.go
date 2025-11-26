package workflow_tasks

import (
	"go.temporal.io/sdk/temporal"

	"temporal-poc/src/core/domain"
)

// WorkflowTaskOptions holds optional configuration for workflow task registration
type WorkflowTaskOptions struct {
	RetryPolicy *temporal.RetryPolicy
	Schema      *domain.NodeSchema
	Visibility  string // "public" or "internal" (default: "public")
}

// WithRetryPolicyWorkflowTask sets the retry policy option for workflow tasks
func WithRetryPolicyWorkflowTask(retryPolicy *temporal.RetryPolicy) func(*WorkflowTaskOptions) {
	return func(opts *WorkflowTaskOptions) {
		opts.RetryPolicy = retryPolicy
	}
}

// WithSchemaWorkflowTask sets the schema option for workflow tasks
func WithSchemaWorkflowTask(schema *domain.NodeSchema) func(*WorkflowTaskOptions) {
	return func(opts *WorkflowTaskOptions) {
		opts.Schema = schema
	}
}

// WithPublicVisibilityWorkflowTask sets visibility to "public" for workflow tasks
func WithPublicVisibilityWorkflowTask() func(*WorkflowTaskOptions) {
	return func(opts *WorkflowTaskOptions) {
		opts.Visibility = "public"
	}
}

// WithInternalVisibilityWorkflowTask sets visibility to "internal" for workflow tasks
func WithInternalVisibilityWorkflowTask() func(*WorkflowTaskOptions) {
	return func(opts *WorkflowTaskOptions) {
		opts.Visibility = "internal"
	}
}
