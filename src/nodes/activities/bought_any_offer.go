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

const BoughtAnyOfferActivityName = "bought_any_offer"

// BoughtAnyOfferSchema defines the input schema for bought_any_offer activity
type BoughtAnyOfferSchema struct {
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
		SchemaStruct: BoughtAnyOfferSchema{},
	}

	RegisterActivity(
		BoughtAnyOfferActivityName,
		BoughtAnyOfferActivity,
		WithRetryPolicy(retryPolicy),
		WithSchema(schema),
		WithPublicVisibility(),
	)
}

// BoughtAnyOfferActivity checks if the user bought any offer in the last N minutes
// This activity fakes a database call and returns an event type based on the result
// If condition is satisfied (offer found), returns condition_satisfied (notify creator)
// If condition is not satisfied (no offer found), returns condition_not_satisfied (move to step 3)
func BoughtAnyOfferActivity(ctx context.Context, activityCtx ActivityContext, deps core.Deps) (ActivityResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("BoughtAnyOfferActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Unmarshal schema
	schema, err := helpers.UnmarshalSchema[BoughtAnyOfferSchema](activityCtx.Schema)
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

	logger.Info("Checking for offers bought in last minutes", "last_minutes", schema.LastMinutes)

	// Fake database call - simulate querying the database
	// In a real implementation, this would query the actual database
	logger.Info("FAKE DATABASE: Querying database for offers bought in last minutes", "last_minutes", schema.LastMinutes)

	activity_helpers.SleepWithHeartbeat(ctx, 1500*time.Millisecond, 500*time.Millisecond)

	// Fake database result - for demonstration, we'll randomly determine if an offer was found
	// In a real implementation, this would be based on actual database query results
	// For this POC, we use random probability: 20% chance of offer found (satisfied), 80% chance not found (not satisfied)
	randomValue := rand.Intn(101)  // Random number from 0 to 100
	offerFound := randomValue < 20 // 20% chance (0-19 = satisfied, 20-99 = not satisfied)

	logger.Info("FAKE DATABASE: Random check result", "random_value", randomValue, "offer_found", offerFound)

	if offerFound {
		logger.Info("FAKE DATABASE: Offer found in last minutes", "last_minutes", schema.LastMinutes)
		logger.Info("BoughtAnyOfferActivity: Condition satisfied - offer found")
		// Return condition_satisfied to notify creator
		return ActivityResult{
			EventType: domain.EventTypeConditionSatisfied,
		}, nil
	}

	logger.Info("FAKE DATABASE: No offer found in last minutes", "last_minutes", schema.LastMinutes)
	logger.Info("BoughtAnyOfferActivity: Condition not satisfied - no offer found")
	// Return condition_not_satisfied to move to step 3
	return ActivityResult{
		EventType: domain.EventTypeConditionNotSatisfied,
	}, nil
}
