package activities

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/helpers"
	"temporal-poc/src/services"
)

const StartRecoveryCartActivityName = "start_recovery_cart"

// StartRecoveryCartSchema defines the input schema for start_recovery_cart activity
type StartRecoveryCartSchema struct {
	UserID    string `json:"user_id" jsonschema:"description=User ID for recovery cart"`
	OfferID   string `json:"offer_id" jsonschema:"description=Offer ID for recovery cart"`
	CreatorId string `json:"creator_id" jsonschema:"description=Creator ID for recovery cart"`
	CartID    string `json:"cart_id" jsonschema:"description=Cart ID for recovery cart"`
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
		SchemaStruct: StartRecoveryCartSchema{},
	}

	RegisterActivity(StartRecoveryCartActivityName, StartRecoveryCartActivity, retryPolicy, schema)
}

// StartRecoveryCartActivity simulates a call to an internal API to start a recovery cart
// This activity calls the leads service to mock an internal API call
func StartRecoveryCartActivity(ctx context.Context, activityCtx ActivityContext, deps core.Deps) (ActivityResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("StartRecoveryCartActivity executing", "workflow_id", activityCtx.WorkflowID)

	// Unmarshal schema
	schema, err := helpers.UnmarshalSchema[StartRecoveryCartSchema](activityCtx.Schema)
	if err != nil {
		logger.Error("Failed to unmarshal schema", "error", err)
		return ActivityResult{}, temporal.NewApplicationError(
			fmt.Sprintf("invalid schema: %v", err),
			"InvalidSchema",
		)
	}

	logger.Info("Calling internal API to start recovery cart", "workflow_id", activityCtx.WorkflowID, "user_id", schema.UserID)

	// Prepare request for the internal API
	response, err := deps.LeadService.StartRecoveryCart(ctx, services.StartRecoveryCartRequest{
		WorkflowID: activityCtx.WorkflowID,
		UserID:     schema.UserID,
		CreatorId:  schema.CreatorId,
		OfferID:    schema.OfferID,
		CartID:     schema.CartID,
	})

	if err != nil {
		logger.Error("INTERNAL API: Failed to start recovery cart", "error", err)
		// Return error to trigger retries
		return ActivityResult{}, temporal.NewApplicationError(
			fmt.Sprintf("failed to start recovery cart: %v", err),
			"InternalAPIError",
		)
	}

	logger.Info("INTERNAL API: Recovery cart started successfully",
		"message", response.Message,
		"timestamp", response.Timestamp,
	)

	logger.Info("StartRecoveryCartActivity completed successfully")

	// Return success with condition_satisfied event type
	return ActivityResult{
		EventType: domain.EventTypeConditionSatisfied,
		Metadata: map[string]interface{}{
			"message":   response.Message,
			"timestamp": response.Timestamp,
		},
	}, nil
}
