package nodes

import (
	"go.temporal.io/sdk/workflow"
)

// ClientAnsweredProcessorHandler handles the client-answered signal by calling workflow and upserting search attributes
type ClientAnsweredProcessorHandler struct {
	BaseHandler
}

func NewClientAnsweredProcessorHandler() *ClientAnsweredProcessorHandler {
	return &ClientAnsweredProcessorHandler{}
}

func (h *ClientAnsweredProcessorHandler) Handle(ctx workflow.Context, handlerCtx *HandlerContext, selector workflow.Selector) HandlerResult {
	handlerCtx.Logger.Info("Processing client-answered: calling workflow and upserting search attributes")

	// Upsert search attributes to mark client-answered
	// Search attributes are indexed and searchable, persisting in the visibility store
	// They will be available for queries months from now as long as the workflow execution exists
	err := workflow.UpsertTypedSearchAttributes(
		ctx,
		ClientAnsweredField.ValueSet(true),
		ClientAnsweredAtField.ValueSet(workflow.Now(ctx).UTC()),
	)
	if err != nil {
		handlerCtx.Logger.Error("Failed to upsert search attributes", "error", err)
	} else {
		handlerCtx.Logger.Info("Successfully upserted client-answered search attributes (searchable and persistent)")
	}

	// Optionally call a child workflow or activity here
	// Example: ExecuteActivity to process the client-answered event
	// For now, just log that we would call a workflow
	handlerCtx.Logger.Info("Would call workflow to process client-answered event")

	// Update the handler context
	handlerCtx.ClientAnswered = true

	// Continue to next handler if exists
	if h.next != nil {
		return h.next.Handle(ctx, handlerCtx, selector)
	}
	return Stop
}
