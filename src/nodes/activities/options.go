package activities

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
