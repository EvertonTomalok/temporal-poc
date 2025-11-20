package nodes

import (
	"go.temporal.io/sdk/workflow"
)

// ClientAnsweredHandler optionally waits for the "client-answered" signal and handles timeout
type ClientAnsweredHandler struct {
	BaseHandler
	channel workflow.ReceiveChannel
}

func NewClientAnsweredHandler(channel workflow.ReceiveChannel) *ClientAnsweredHandler {
	return &ClientAnsweredHandler{
		channel: channel,
	}
}

func (h *ClientAnsweredHandler) Handle(ctx workflow.Context, handlerCtx *HandlerContext, selector workflow.Selector) HandlerResult {
	// If already answered, stop processing
	if handlerCtx.ClientAnswered {
		return Stop
	}

	// Wait for client-answered signal
	// The selector will handle both buffered and future signals
	selector.AddReceive(h.channel, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil) // Receive the signal (no payload needed)
		handlerCtx.Logger.Info("client-answered signal received - completing workflow")
		handlerCtx.ClientAnswered = true
	})

	// Handle timeout: calculate remaining time
	elapsed := workflow.Now(ctx).Sub(handlerCtx.StartTime)
	remaining := handlerCtx.TimeoutDuration - elapsed

	if remaining <= 0 {
		handlerCtx.Logger.Info("1 minute timeout reached")
		return Stop
	}

	// Only add timer if one doesn't already exist
	if handlerCtx.Timer == nil {
		// Add timer for remaining time
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		timer := workflow.NewTimer(timerCtx, remaining)
		handlerCtx.Timer = timer
		handlerCtx.CancelTimer = cancelTimer

		selector.AddFuture(timer, func(f workflow.Future) {
			cancelTimer()
			handlerCtx.Logger.Info("Timeout timer fired")
		})
	}

	// Continue to next handler
	if h.next != nil {
		return h.next.Handle(ctx, handlerCtx, selector)
	}
	return Continue
}
