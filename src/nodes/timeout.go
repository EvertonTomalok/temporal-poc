package nodes

import (
	"go.temporal.io/sdk/workflow"
)

// TimeoutHandler handles the timeout scenario
type TimeoutHandler struct {
	BaseHandler
}

func NewTimeoutHandler() *TimeoutHandler {
	return &TimeoutHandler{}
}

func (h *TimeoutHandler) Handle(ctx workflow.Context, handlerCtx *HandlerContext, selector workflow.Selector) HandlerResult {
	// Calculate remaining time
	elapsed := workflow.Now(ctx).Sub(handlerCtx.StartTime)
	remaining := handlerCtx.TimeoutDuration - elapsed

	if remaining <= 0 {
		handlerCtx.Logger.Info("1 minute timeout reached")
		return Stop
	}

	// Add timer for remaining time
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(timerCtx, remaining)
	handlerCtx.Timer = timer
	handlerCtx.CancelTimer = cancelTimer

	selector.AddFuture(timer, func(f workflow.Future) {
		cancelTimer()
	})

	// Continue to next handler
	if h.next != nil {
		return h.next.Handle(ctx, handlerCtx, selector)
	}
	return Continue
}
