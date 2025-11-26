package activities

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	activity_helpers "temporal-poc/src/nodes/activities/helpers"
)

const UpdateDatabaseAttributesActivityName = "update_database_attributes"

// UpdateDatabaseAttributesSchema defines the input schema for update database attributes activity
type UpdateDatabaseAttributesSchema struct {
	SleepDuration string                 `json:"sleep_duration,omitempty" jsonschema:"description=Duration to sleep (e.g., '3s', '500ms'),default=1s"`
	Attributes    map[string]interface{} `json:"attributes,omitempty" jsonschema:"description=Optional database attributes to update"`
}

func init() {
	// Register with retry policy for automatic retries on failure
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    15,
	}
	// Define schema for validation
	schema := &domain.NodeSchema{
		SchemaStruct: UpdateDatabaseAttributesSchema{},
	}
	RegisterActivity(
		UpdateDatabaseAttributesActivityName,
		UpdateDatabaseAttributesActivity,
		WithRetryPolicy(retryPolicy),
		WithSchema(schema),
		WithPublicVisibility(),
	)
}

// UpdateDatabaseAttributesActivity updates database attributes
// This activity mocks a database update operation with a configurable sleep time
// It always succeeds (no failures)
func UpdateDatabaseAttributesActivity(ctx context.Context, activityCtx ActivityContext, deps core.Deps) (ActivityResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("UpdateDatabaseAttributesActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Parse sleep duration from schema, default to 2 seconds
	sleepDuration := 1 * time.Second
	if activityCtx.Schema != nil {
		if sleepStr, ok := activityCtx.Schema["sleep_duration"].(string); ok && sleepStr != "" {
			if parsed, err := time.ParseDuration(sleepStr); err == nil {
				sleepDuration = parsed
			}
		}
	}

	logger.Info("UpdateDatabaseAttributesActivity: Starting database update",
		"sleep_duration", sleepDuration.String(),
		"workflow_id", activityCtx.WorkflowID)

	// Mock database update operation with sleep
	// Use SleepWithHeartbeat to keep the activity alive during the operation
	// Heartbeat every 500ms to keep the activity responsive
	activity_helpers.SleepWithHeartbeat(ctx, sleepDuration, 500*time.Millisecond)

	logger.Info("UpdateDatabaseAttributesActivity: Database update completed successfully")
	logger.Info("UpdateDatabaseAttributesActivity completed successfully")

	return ActivityResult{
		EventType: domain.EventTypeConditionSatisfied,
		Metadata: map[string]interface{}{
			"updated":        true,
			"sleep_duration": sleepDuration.String(),
		},
	}, nil
}
