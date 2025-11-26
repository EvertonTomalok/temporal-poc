package nodes

import (
	"go.temporal.io/sdk/temporal"

	"temporal-poc/src/core/domain"
)

// ActivityOptions holds optional configuration for activity registration
type ActivityOptions struct {
	RetryPolicy *temporal.RetryPolicy
	Schema      *domain.NodeSchema
	Visibility  string // "public" or "internal" (default: "public")
}

// WorkflowTaskOptions holds optional configuration for workflow task registration
type WorkflowTaskOptions struct {
	RetryPolicy *temporal.RetryPolicy
	Schema      *domain.NodeSchema
	Visibility  string // "public" or "internal" (default: "public")
}

// WithRetryPolicy sets the retry policy option
func WithRetryPolicy(retryPolicy *temporal.RetryPolicy) func(*ActivityOptions) {
	return func(opts *ActivityOptions) {
		opts.RetryPolicy = retryPolicy
	}
}

// WithSchema sets the schema option
func WithSchema(schema *domain.NodeSchema) func(*ActivityOptions) {
	return func(opts *ActivityOptions) {
		opts.Schema = schema
	}
}

// WithPublicVisibility sets visibility to "public"
func WithPublicVisibility() func(*ActivityOptions) {
	return func(opts *ActivityOptions) {
		opts.Visibility = "public"
	}
}

// WithInternalVisibility sets visibility to "internal"
func WithInternalVisibility() func(*ActivityOptions) {
	return func(opts *ActivityOptions) {
		opts.Visibility = "internal"
	}
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
