package activities

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/helpers"
	activity_helpers "temporal-poc/src/nodes/activities/helpers"
)

const IsConvertedActivityName = "is_converted"

// IsConvertedSchema defines the input schema for is_converted activity
type IsConvertedSchema struct {
	LastMinutes int64 `json:"last_minutes" jsonschema:"description=Number of minutes to check,required,minimum=0"` // Required: number of minutes to check
}

func init() {
	// Register with retry policy for automatic retries on failure
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2,
		MaximumInterval:    45 * time.Second,
		MaximumAttempts:    20,
	}

	// Define schema for validation
	schema := &domain.NodeSchema{
		SchemaStruct: IsConvertedSchema{},
	}

	RegisterActivity(
		IsConvertedActivityName,
		IsConvertedActivity,
		WithRetryPolicy(retryPolicy),
		WithSchema(schema),
		WithPublicVisibility(),
	)
}

// IsConvertedActivity checks if the user is converted in the last N minutes
// This activity fakes a database call and returns an event type based on the result
// If condition is satisfied (conversion found), returns condition_satisfied (notify creator)
// If condition is not satisfied (no conversion found), returns condition_not_satisfied (move to step 3)
func IsConvertedActivity(ctx context.Context, activityCtx ActivityContext, deps core.Deps) (ActivityResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("IsConvertedActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Unmarshal schema
	schema, err := helpers.UnmarshalSchema[IsConvertedSchema](activityCtx.Schema)
	if err != nil {
		logger.Error("Failed to unmarshal schema", "error", err)
		return ActivityResult{}, temporal.NewApplicationError(
			fmt.Sprintf("invalid schema: %v", err),
			"InvalidSchema",
		)
	}

	// Validate that last_minutes is provided and positive
	if schema.LastMinutes <= 0 {
		logger.Error("last_minutes must be a positive number", "last_minutes", schema.LastMinutes)
		return ActivityResult{}, temporal.NewApplicationError(
			"last_minutes must be a positive number",
			"InvalidSchema",
		)
	}

	logger.Info("Checking for conversions in last minutes", "last_minutes", schema.LastMinutes)

	// Fake database call - simulate querying the database
	// In a real implementation, this would query the actual database
	logger.Info("FAKE DATABASE: Querying database for conversions in last minutes", "last_minutes", schema.LastMinutes)

	activity_helpers.SleepWithHeartbeat(ctx, 1500*time.Millisecond, 500*time.Millisecond)

	// Fake database result - for demonstration, we'll randomly determine if a conversion was found
	// In a real implementation, this would be based on actual database query results
	// For this POC, we use random probability: 20% chance of conversion found (satisfied), 80% chance not found (not satisfied)
	randomValue := rand.Intn(101)       // Random number from 0 to 100
	conversionFound := randomValue < 20 // 20% chance (0-19 = satisfied, 20-99 = not satisfied)

	logger.Info("FAKE DATABASE: Random check result", "random_value", randomValue, "conversion_found", conversionFound)

	if conversionFound {
		logger.Info("FAKE DATABASE: Conversion found in last minutes", "last_minutes", schema.LastMinutes)
		logger.Info("IsConvertedActivity: Condition satisfied - conversion found")
		// Return condition_satisfied to notify creator
		return ActivityResult{
			EventType: domain.EventTypeConditionSatisfied,
		}, nil
	}

	logger.Info("FAKE DATABASE: No conversion found in last minutes", "last_minutes", schema.LastMinutes)
	logger.Info("IsConvertedActivity: Condition not satisfied - no conversion found")
	// Return condition_not_satisfied to move to step 3
	return ActivityResult{
		EventType: domain.EventTypeConditionNotSatisfied,
	}, nil
}
