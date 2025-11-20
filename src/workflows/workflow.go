package src

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/nodes"
)

// SignalCollectorWorkflow waits for 1 minute or completes when it receives the "client-answered" signal
// Uses Chain of Responsibility pattern where nodes handle signals
func SignalCollectorWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("SignalCollectorWorkflow started")

	// Create channel for client-answered signal
	clientAnsweredChannel := workflow.GetSignalChannel(ctx, "client-answered")

	// Initialize handler context
	handlerCtx := &nodes.HandlerContext{
		ClientAnswered:  false,
		StartTime:       workflow.Now(ctx),
		TimeoutDuration: 1 * time.Minute,
		Logger:          logger,
	}

	// Build the chain of responsibility
	chain := nodes.NewHandlerChain()

	// Handlers that can optionally wait for signals
	chain.AddHandler(nodes.NewClientAnsweredHandler(clientAnsweredChannel))
	chain.AddHandler(nodes.NewTimeoutHandler())

	// Listen for signals until timeout or client-answered is received
	for {
		// Calculate remaining time
		elapsed := workflow.Now(ctx).Sub(handlerCtx.StartTime)
		remaining := handlerCtx.TimeoutDuration - elapsed

		if remaining <= 0 {
			logger.Info("1 minute timeout reached")
			break
		}

		// Use selector to wait for signal or timeout
		selector := workflow.NewSelector(ctx)

		// Process through the chain - nodes will add their handlers to the selector
		chain.Process(ctx, handlerCtx, selector)

		// Wait for either signal or timeout
		selector.Select(ctx)

		// Check if workflow should stop
		if handlerCtx.ClientAnswered {
			if handlerCtx.CancelTimer != nil {
				handlerCtx.CancelTimer()
			}
			logger.Info("Workflow completing due to client-answered signal")
			break
		}

		// Check if timer fired (timeout reached)
		if handlerCtx.Timer != nil && handlerCtx.Timer.IsReady() {
			if handlerCtx.CancelTimer != nil {
				handlerCtx.CancelTimer()
			}
			logger.Info("1 minute timeout reached")
			break
		}

		// If signal was received, cancel the timer to avoid resource leak
		if handlerCtx.CancelTimer != nil {
			handlerCtx.CancelTimer()
		}
	}

	logger.Info("Workflow finishing")
	return nil
}
