package activities

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

// SleepWithHeartbeat sleeps for the specified duration while sending periodic heartbeats
// to keep the activity alive. This is useful for long-running activities that need
// to report progress and avoid timeout.
//
// Parameters:
//   - ctx: The activity context
//   - sleepTime: Total duration to sleep
//   - heartbeatInterval: Interval between heartbeats (must be less than sleepTime)
//
// If heartbeatInterval is 0 or greater than sleepTime, it will send a heartbeat
// at the start and end only.
func SleepWithHeartbeat(ctx context.Context, sleepTime time.Duration, heartbeatInterval time.Duration) {
	if sleepTime <= 0 {
		return
	}

	// If heartbeat interval is invalid, just sleep without heartbeats
	if heartbeatInterval <= 0 || heartbeatInterval >= sleepTime {
		time.Sleep(sleepTime)
		activity.RecordHeartbeat(ctx)
		return
	}

	// Send initial heartbeat
	activity.RecordHeartbeat(ctx)

	// Sleep in chunks, sending heartbeats periodically
	elapsed := time.Duration(0)
	for elapsed < sleepTime {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Calculate how long to sleep in this iteration
		remaining := sleepTime - elapsed
		sleepDuration := heartbeatInterval
		if remaining < heartbeatInterval {
			sleepDuration = remaining
		}

		// Sleep for the interval (or remaining time)
		time.Sleep(sleepDuration)
		elapsed += sleepDuration

		// Send heartbeat after each interval
		activity.RecordHeartbeat(ctx)
	}
}
