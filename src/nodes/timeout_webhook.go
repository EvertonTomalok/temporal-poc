package nodes

import (
	"go.temporal.io/sdk/workflow"
)

// TimeoutWebhookHandler handles timeout by faking a webhook call (just prints)
type TimeoutWebhookHandler struct {
	BaseHandler
}

func NewTimeoutWebhookHandler() *TimeoutWebhookHandler {
	return &TimeoutWebhookHandler{}
}

func (h *TimeoutWebhookHandler) Handle(ctx workflow.Context, handlerCtx *HandlerContext, selector workflow.Selector) HandlerResult {
	handlerCtx.Logger.Info("Timeout reached - faking webhook call")

	// Fake webhook call - just print/log
	handlerCtx.Logger.Info("FAKE WEBHOOK CALL: POST /webhook/timeout")
	handlerCtx.Logger.Info("FAKE WEBHOOK PAYLOAD: { \"event\": \"timeout\", \"workflow_id\": \"" + workflow.GetInfo(ctx).WorkflowExecution.ID + "\" }")
	handlerCtx.Logger.Info("FAKE WEBHOOK RESPONSE: 200 OK")

	// Update memo to record timeout event
	memo := map[string]interface{}{
		"timeout_occurred": true,
		"timeout_at":       workflow.Now(ctx).UTC(),
		"event":            "timeout",
		"workflow_id":      workflow.GetInfo(ctx).WorkflowExecution.ID,
	}
	err := workflow.UpsertMemo(ctx, memo)
	if err != nil {
		handlerCtx.Logger.Error("Failed to upsert memo", "error", err)
	} else {
		handlerCtx.Logger.Info("Successfully updated memo with timeout information")
	}

	// Continue to next handler if exists
	if h.next != nil {
		return h.next.Handle(ctx, handlerCtx, selector)
	}
	return Stop
}
