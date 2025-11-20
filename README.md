# Temporal Abandoned Cart POC

This is a Proof of Concept demonstrating Temporal workflows that wait for a "client-answered" signal or timeout after 1 minute. The workflow uses the Chain of Responsibility design pattern to handle different scenarios.

## Overview

The workflow starts and waits for either:
- A "client-answered" signal (which causes immediate completion)
- A 1-minute timeout (which causes completion after the timeout)

The workflow uses the Chain of Responsibility pattern where different handlers process signals and timeouts.

## Structure

- `src/workflows/workflow.go` - Defines the `AbandonedCartWorkflow` that waits for signals
- `src/nodes/` - Contains handler implementations using Chain of Responsibility pattern:
  - `handler.go` - Base handler interface and chain management
  - `client_answered.go` - Handler for "client-answered" signal
  - `timeout.go` - Handler for timeout scenario
- `cmd/worker/main.go` - Contains the worker that processes workflow tasks
- `cmd/client/main.go` - Contains the client that starts workflows and waits for completion
- `cmd/signal-sender/main.go` - Utility to send "client-answered" signal to a running workflow

## Prerequisites

- Go 1.23 or later
- Temporal server running (default: localhost:7233)

### Registering Search Attributes

For search attributes to work (used for persistent, searchable client-answered tracking), you need to register them with Temporal server. When starting Temporal server, add:

```bash
temporal server start-dev \
  --search-attribute ClientAnswered=Bool \
  --search-attribute ClientAnsweredAt=Datetime
```

Or if using docker-compose, add these to your Temporal server configuration.

**Note**: Search attributes are indexed and searchable, persisting in the visibility store (Elasticsearch/OpenSearch) for as long as the workflow execution exists. This makes them suitable for long-term searches (months from now), unlike memos which are not searchable and are deleted when workflows terminate.

## Running the POC

### 1. Start the Worker

In one terminal, start the worker:

```bash
go run ./cmd/worker
```

The worker will listen on the task queue `primary-workflow-task-queue`.

### 2. Run the Client

In another terminal, run the client to start a workflow:

```bash
go run ./cmd/client
```

The client will:
1. Start a new workflow
2. Wait for the workflow to complete (either by receiving "client-answered" signal or after 1 minute timeout)
3. Display the workflow ID and completion status

### 3. Send Signal (Optional)

In a third terminal, you can send the "client-answered" signal to complete the workflow early:

```bash
go run ./cmd/signal-sender -workflow-id <WORKFLOW_ID>
```

Replace `<WORKFLOW_ID>` with the workflow ID printed by the client.

## How It Works

1. The workflow starts and initializes a Chain of Responsibility with two handlers:
   - `ClientAnsweredHandler` - Listens for the "client-answered" signal
   - `TimeoutHandler` - Sets up a timer for the 1-minute timeout

2. The workflow enters a loop that:
   - Calculates remaining time until timeout
   - Uses a selector to wait for either:
     - The "client-answered" signal (via ClientAnsweredHandler)
     - The timeout timer (via TimeoutHandler)

3. When the "client-answered" signal is received:
   - The ClientAnsweredHandler sets `ClientAnswered = true`
   - The workflow breaks out of the loop and completes immediately

4. If the timeout is reached:
   - The TimeoutHandler's timer fires
   - The workflow breaks out of the loop and completes

5. The client receives the workflow completion and displays the result

## Chain of Responsibility Pattern

This POC demonstrates the Chain of Responsibility design pattern in Temporal workflows:

- **Handler Interface**: Each handler implements `SignalHandler` with a `Handle()` method
- **Handler Chain**: Handlers are linked together, processing in sequence
- **Selector Integration**: Each handler can add its own selectors (signal channels, timers) to the workflow selector
- **Flexible Processing**: New handlers can be added to the chain without modifying existing code

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
   - Local variables (like `handlerCtx.StartTime`, `handlerCtx.ClientAnswered`) are recomputed from the history

### State Reconstruction

In this workflow, all state is automatically reconstructed during replay:

- `handlerCtx.StartTime` - Set from the workflow start event
- `handlerCtx.ClientAnswered` - Set when the "client-answered" signal event is replayed
- `handlerCtx.Timer` - Derived from timer events in the history
- Handler chain state - Rebuilt by replaying signal and timer events in order

### Example: Worker Restart Scenario

```
Worker A running workflow:
  - Starts workflow, initializes handler chain
  - Sets up timeout timer
  - Receives "client-answered" signal
  - ClientAnsweredHandler processes signal → ClientAnswered = true
  - Worker A crashes 💥

Worker B picks up the workflow:
  - Temporal sends workflow task with history
  - Worker B replays from beginning:
    1. Replays workflow start event → StartTime set
    2. Replays handler chain initialization
    3. Replays timeout timer setup
    4. Replays "client-answered" signal event → ClientAnswered = true
    5. Continues from where Worker A left off
  - Worker B completes workflow seamlessly
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

### Client Output

```
Workflow ID: abandonedCart-abc123
Started workflow with ID: abandonedCart-abc123 and RunID: xyz789
Waiting for 'client-answered' signal (max 1 minute)...
Received 'client-answered' signal - workflow completed
```

Or if timeout occurs:

```
Workflow ID: abandonedCart-abc123
Started workflow with ID: abandonedCart-abc123 and RunID: xyz789
Waiting for 'client-answered' signal (max 1 minute)...
Timeout: Did not receive 'client-answered' signal within 1 minute
```

### Signal Sender Output

```
Successfully sent 'client-answered' signal to workflow: abandonedCart-abc123
```