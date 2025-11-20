package workflows

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// SignalMessage represents a signal with its message
type SignalMessage struct {
	SignalName string
	Message    string
	ReceivedAt time.Time
}

// SignalCollectorWorkflow waits for 1 minute to collect signals and their messages
// or completes when it receives the "client-answered" signal
func SignalCollectorWorkflow(ctx workflow.Context) ([]SignalMessage, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("SignalCollectorWorkflow started")

	// Create channels for different signals
	collectSignalChannel := workflow.GetSignalChannel(ctx, "collect-signal")
	clientAnsweredChannel := workflow.GetSignalChannel(ctx, "client-answered")

	// Slice to store collected signals
	var collectedSignals []SignalMessage

	// Start time to track elapsed time
	startTime := workflow.Now(ctx)
	timeoutDuration := 1 * time.Minute
	clientAnswered := false

	// Listen for signals until 1 minute has passed or client-answered is received
	for {
		// Calculate remaining time
		elapsed := workflow.Now(ctx).Sub(startTime)
		remaining := timeoutDuration - elapsed

		if remaining <= 0 {
			logger.Info("1 minute timeout reached")
			break
		}

		// Use selector to wait for signal or timeout
		selector := workflow.NewSelector(ctx)

		// Add collect-signal channel
		selector.AddReceive(collectSignalChannel, func(c workflow.ReceiveChannel, more bool) {
			var message string
			c.Receive(ctx, &message)

			signalMsg := SignalMessage{
				SignalName: "collect-signal",
				Message:    message,
				ReceivedAt: workflow.Now(ctx),
			}
			collectedSignals = append(collectedSignals, signalMsg)
			logger.Info("Signal received", "message", message, "total", len(collectedSignals))
		})

		// Add client-answered channel
		selector.AddReceive(clientAnsweredChannel, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, nil) // Receive the signal (no payload needed)
			logger.Info("client-answered signal received - completing workflow")
			clientAnswered = true
		})

		// Add timer for remaining time
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		timer := workflow.NewTimer(timerCtx, remaining)
		selector.AddFuture(timer, func(f workflow.Future) {
			cancelTimer()
		})

		// Wait for either signal or timeout
		selector.Select(ctx)

		// Check if client-answered was received
		if clientAnswered {
			cancelTimer()
			logger.Info("Workflow completing due to client-answered signal")
			break
		}

		// Check if timer fired (timeout reached)
		if timer.IsReady() {
			cancelTimer()
			logger.Info("1 minute timeout reached")
			break
		}
		// If signal was received, cancel the timer to avoid resource leak
		cancelTimer()
	}

	logger.Info("Workflow finishing", "total_signals", len(collectedSignals))
	return collectedSignals, nil
}
