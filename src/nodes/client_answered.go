package nodes

import (
	"go.temporal.io/sdk/workflow"
)

// ClientAnsweredHandler optionally waits for the "client-answered" signal
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
	// Optionally wait for client-answered signal
	selector.AddReceive(h.channel, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil) // Receive the signal (no payload needed)
		handlerCtx.Logger.Info("client-answered signal received - completing workflow")
		handlerCtx.ClientAnswered = true
	})

	// Continue to next handler
	if h.next != nil {
		return h.next.Handle(ctx, handlerCtx, selector)
	}
	return Continue
}
