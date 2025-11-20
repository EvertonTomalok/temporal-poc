# Temporal Signal Collector POC

This is a Proof of Concept demonstrating Temporal workflows that collect signals with messages over a 1-minute period.

## Overview

The workflow starts and waits for 1 minute to collect signals and their associated messages. After 1 minute, the workflow completes and returns all collected signals.

## Structure

- `workflows/workflow.go` - Defines the `SignalCollectorWorkflow` that waits for signals
- `worker/main.go` - Contains the worker that processes workflow tasks
- `client/main.go` - Contains the client that starts workflows and sends signals

## Prerequisites

- Go 1.23 or later
- Temporal server running (default: localhost:7233)

## Running the POC

### 1. Start the Worker

In one terminal, start the worker:

```bash
go run ./worker
```

The worker will listen on the task queue `signal-collector-task-queue`.

### 2. Run the Client

In another terminal, run the client to start a workflow and send signals:

```bash
go run ./client
```

The client will:
1. Start a new workflow
2. Send 4 signals with messages (5 seconds apart)
3. Wait for the workflow to complete (after 1 minute)
4. Display all collected signals

## How It Works

1. The workflow starts and begins listening for signals on the `collect-signal` channel
2. It uses a selector to wait for either:
   - A signal arriving with a message
   - A 1-minute timeout
3. When a signal is received, it's stored with its message and timestamp
4. After 1 minute, the workflow completes and returns all collected signals
5. The client receives the results and displays them

## State Safety and Worker Restarts

Temporal guarantees that workflow state is preserved even when workers restart or when another worker picks up the workflow. This is achieved through **event sourcing** and **deterministic replay**.

### Core Mechanism: Event History

- **Event History as Source of Truth**: Temporal stores all workflow state as an immutable sequence of events in the database (not in worker memory). Each event represents a state change: workflow started, signal received, timer fired, etc.

- **Deterministic Execution**: Workflows must be deterministic - given the same event history, they produce the same result. The Temporal SDK ensures all workflow APIs (`workflow.Now()`, `workflow.GetSignalChannel()`, `workflow.NewTimer()`, etc.) are deterministic and replayable.

### How Replay Works

When a worker picks up a workflow (or restarts):

1. **Temporal sends the workflow task** with the complete event history
2. **The worker replays the workflow** from the beginning using the history
3. **During replay**:
   - `workflow.Now()` returns the time from history events
   - Signal channels replay signals in the exact order they were received
   - Timers replay based on history events
   - Local variables (like `collectedSignals`, `startTime`, `clientAnswered`) are recomputed from the history

### State Reconstruction

In this workflow, all state is automatically reconstructed during replay:

- `collectedSignals []SignalMessage` - Rebuilt by replaying all signal events in order
- `startTime` - Set from the workflow start event
- `clientAnswered` - Set when the "client-answered" signal event is replayed
- Timer state - Derived from timer events in the history

### Example: Worker Restart Scenario

```
Worker A running workflow:
  - Receives signal "collect-signal" with message "Hello"
  - Appends to collectedSignals
  - Worker A crashes 💥

Worker B picks up the workflow:
  - Temporal sends workflow task with history
  - Worker B replays from beginning:
    1. Replays workflow start event
    2. Replays "collect-signal" event → collectedSignals = [SignalMessage{...}]
    3. Continues from where Worker A left off
  - Worker B continues execution seamlessly
```

### Why This Works

- **No External State**: Workflows don't rely on worker memory, files, or external systems
- **Deterministic APIs**: All Temporal workflow APIs are deterministic and replayable
- **Immutable History**: Once written, events never change
- **Automatic Recovery**: Temporal automatically reassigns tasks to available workers

### Important Constraints

- **Only use Temporal SDK APIs** inside workflows (no `time.Now()`, no random numbers, no external I/O)
- **Keep workflow code deterministic** (no goroutines, no channels, no maps with random iteration)
- **State must be reconstructable** from history events

This is why Temporal workflows are durable and fault-tolerant: **state lives in the history, not in worker memory**.

## Example Output

```
Started workflow with ID: signal-collector-xxx and RunID: yyy
Sending signals...
Sent signal 1: Hello from signal 1
Sent signal 2: Hello from signal 2
Sent signal 3: Hello from signal 3
Sent signal 4: Hello from signal 4
Waiting for workflow to complete...

=== Workflow Results ===
Total signals collected: 4
Signal 1: Hello from signal 1 (received at: 2024-01-01T12:00:00Z)
Signal 2: Hello from signal 2 (received at: 2024-01-01T12:00:05Z)
Signal 3: Hello from signal 3 (received at: 2024-01-01T12:00:10Z)
Signal 4: Hello from signal 4 (received at: 2024-01-01T12:00:15Z)
```