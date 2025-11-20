package src

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"temporal-poc/src/nodes"
)

// AbandonedCartWorkflow waits for 1 minute or completes when it receives the "client-answered" signal
// Uses Chain of Responsibility pattern where nodes handle signals
func AbandonedCartWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("AbandonedCartWorkflow started")

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

	// Handler that waits for client-answered signal and handles timeout
	chain.AddHandler(nodes.NewClientAnsweredHandler(clientAnsweredChannel))

	// Listen for signals until timeout or client-answered is received
	for {
		// Check if client already answered (before processing handlers)
		if handlerCtx.ClientAnswered {
			if handlerCtx.CancelTimer != nil {
				handlerCtx.CancelTimer()
			}
			logger.Info("Workflow completing due to client-answered signal")
			break
		}

		// Calculate remaining time
		elapsed := workflow.Now(ctx).Sub(handlerCtx.StartTime)
		remaining := handlerCtx.TimeoutDuration - elapsed

		if remaining <= 0 {
			logger.Info("1 minute timeout reached")
			break
		}

		// Use selector to wait for signal or timeout
		// Create a new selector each iteration (like the await-signals example)
		selector := workflow.NewSelector(ctx)

		// Process through the chain - nodes will add their handlers to the selector
		result := chain.Process(ctx, handlerCtx, selector)

		// If handler returned Stop, break immediately
		if result == nodes.Stop {
			if handlerCtx.CancelTimer != nil {
				handlerCtx.CancelTimer()
			}
			break
		}

		// Wait for either signal or timeout
		// This blocks until one of the registered callbacks is triggered
		selector.Select(ctx)

		// After Select() returns, check if we should stop
		// The callbacks have already executed, so check the state
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
	}

	logger.Info("Workflow finishing")
	return nil
}
