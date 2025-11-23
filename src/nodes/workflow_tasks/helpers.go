package workflow_tasks

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// NewTimerWithSummary creates a new timer with a summary for UI visibility.
// This helper function standardizes the timer creation pattern used across workflow tasks.
func NewTimerWithSummary(ctx workflow.Context, duration time.Duration, summary string) workflow.Future {
	return workflow.NewTimerWithOptions(ctx, duration, workflow.TimerOptions{
		Summary: summary,
	})
}
