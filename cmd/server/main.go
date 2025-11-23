package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"temporal-poc/src/config"
	"temporal-poc/src/core"
	"temporal-poc/src/core/domain"
	"temporal-poc/src/register"
	"temporal-poc/src/validation"
	workflows "temporal-poc/src/workflows"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
)

var temporalClient client.Client

func init() {
	// Load Temporal client configuration from .env file
	clientOptions := config.LoadTemporalClientOptions()
	clientOptions.Logger = core.NewLoggerWithoutWarnings()

	// Create Temporal client on startup
	var err error
	temporalClient, err = client.Dial(clientOptions)
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}

	// Register search attributes if they don't exist
	// This MUST succeed before the server can handle workflow requests
	log.Println("Checking and registering search attributes...")
	if err := core.RegisterSearchAttributesIfNeeded(temporalClient); err != nil {
		log.Printf("ERROR: Failed to register search attributes automatically: %v", err)
		log.Println("")
		log.Println("Workflows will fail with BadSearchAttributes error if search attributes are not registered.")
		log.Println("Please register them manually using one of these methods:")
		log.Println("")
		log.Println("  Method 1 (Temporal CLI):")
		log.Println("    temporal operator search-attributes add -name ClientAnswered -type Bool")
		log.Println("    temporal operator search-attributes add -name ClientAnsweredAt -type Datetime")
		log.Println("")
		log.Println("  Method 2 (Temporal Server startup):")
		log.Println("    temporal server start-dev \\")
		log.Println("      --search-attribute ClientAnswered=Bool \\")
		log.Println("      --search-attribute ClientAnsweredAt=Datetime")
		log.Println("")
		log.Fatalln("Server cannot start without registered search attributes. Exiting.")
	}
	log.Println("✓ Search attributes verified/registered successfully")
}

func main() {
	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/nodes", getNodesHandler)
	e.POST("/start-workflow", startWorkflowHandler)
	e.POST("/send-signal", sendSignalHandler)
	e.GET("/workflow-status/:workflow_id", getWorkflowStatusHandler)

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	port := ":8081"
	serverErrChan := make(chan error, 1)
	go func() {
		log.Printf("Server starting on port %s\n", port)
		log.Println("Endpoints:")
		log.Println("  GET  /nodes - Get all available nodes with schemas")
		log.Println("  POST /start-workflow - Start a new workflow")
		log.Println("  POST /send-signal - Send a signal to a workflow")
		log.Println("  GET  /workflow-status/:workflow_id - Get workflow status and processed steps")
		if err := e.Start(port); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	// Wait for interrupt signal or server error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

		// Create a context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Shutdown the server gracefully
		if err := e.Shutdown(ctx); err != nil {
			log.Printf("Error during server shutdown: %v", err)
		} else {
			log.Println("Server stopped gracefully")
		}

		// Close Temporal client
		if temporalClient != nil {
			temporalClient.Close()
			log.Println("Temporal client closed")
		}
	case err := <-serverErrChan:
		if err != nil {
			log.Fatalln("Server failed to start", err)
		}
	}
}

// StartWorkflowRequest represents the request body for starting a workflow
type StartWorkflowRequest struct {
	// Optional: if not provided, a UUID will be generated
	WorkflowID string `json:"workflow_id,omitempty"`
	// Optional: if not provided or invalid, default config will be used
	// Step inputs are validated against node schemas automatically
	Config *workflows.WorkflowConfig `json:"config,omitempty"`
}

// StartWorkflowResponse represents the response from starting a workflow
type StartWorkflowResponse struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	Message    string `json:"message"`
}

// startWorkflowHandler handles POST requests to start a workflow
func startWorkflowHandler(c echo.Context) error {
	var req StartWorkflowRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// Generate workflow ID if not provided
	workflowID := req.WorkflowID
	if workflowID == "" {
		workflowID = workflows.GenerateWorkflowID()
	}

	// Get workflow config from request or use default
	var workflowConfig workflows.WorkflowConfig
	if req.Config != nil {
		// Use the provided config
		workflowConfig = *req.Config

		// Validate the provided config (includes schema validation for step inputs)
		if err := workflows.ValidateWorkflowConfig(workflowConfig); err != nil {
			// If validation fails, return error
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("Invalid workflow config: %v", err),
			})
		}
	} else {
		// Use default config if not provided
		workflowConfig = workflows.BuildDefaultWorkflowDefinition()
		// Validate default config (includes schema validation)
		if err := workflows.ValidateWorkflowConfig(workflowConfig); err != nil {
			log.Printf("Warning: Default workflow config validation failed: %v", err)
		}
	}

	// Build workflow execution config with the validated config
	execConfig := workflows.WorkflowExecutionConfig{
		Options: client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: domain.PrimaryWorkflowTaskQueue,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1, // No retries
			},
		},
		Config:       workflowConfig,
		WorkflowName: "DynamicWorkflow",
	}
	// Start workflow with retry policy and dynamic workflow
	we, err := temporalClient.ExecuteWorkflow(
		context.Background(),
		execConfig.Options,
		execConfig.WorkflowName,
		execConfig.Config,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Unable to execute workflow: %v", err),
		})
	}

	// Return response
	response := StartWorkflowResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Message:    "Workflow started successfully",
	}

	return c.JSON(http.StatusOK, response)
}

// SendSignalRequest represents the request body for sending a signal
type SendSignalRequest struct {
	WorkflowID string `json:"workflow_id,omitempty"`
	RunID      string `json:"run_id,omitempty"`      // Optional: if empty, signals latest run
	SignalName string `json:"signal_name,omitempty"` // Optional: defaults to "client-answered"
	Message    string `json:"message,omitempty"`     // Optional: message to send with the signal
}

// SendSignalResponse represents the response from sending a signal
type SendSignalResponse struct {
	Message string `json:"message"`
}

// sendSignalHandler handles POST requests to send a signal to a workflow
func sendSignalHandler(c echo.Context) error {
	var req SendSignalRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	if req.WorkflowID == "" && req.RunID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "workflow_id or run_id is required",
		})
	}

	workflowID := req.WorkflowID
	runID := req.RunID

	// If only runID is provided, we need to find the workflowID first and then terminate the workflow
	if workflowID == "" && runID != "" {
		// Find workflow execution by runID
		foundWorkflowID, err := findWorkflowIDByRunID(context.Background(), runID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Unable to find workflow by run_id: %v", err),
			})
		}
		if foundWorkflowID == "" {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": fmt.Sprintf("No workflow found with run_id: %s", runID),
			})
		}
		workflowID = foundWorkflowID
	}

	// Default signal name to "client-answered" if not provided
	signalName := req.SignalName
	if signalName == "" {
		signalName = domain.ClientAnsweredSignal
	}

	// Prepare signal payload
	signalPayload := domain.ClientAnsweredSignalPayload{
		Message: req.Message,
	}

	// Send signal to workflow
	// Use empty RunID to signal the latest run if not provided
	err := temporalClient.SignalWorkflow(context.Background(), workflowID, runID, signalName, signalPayload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Unable to signal workflow: %v", err),
		})
	}

	// Return response
	response := SendSignalResponse{
		Message: fmt.Sprintf("Successfully sent '%s' signal to workflow: %s", signalName, workflowID),
	}

	return c.JSON(http.StatusOK, response)
}

// findWorkflowIDByRunID searches for a workflow execution by runID and returns its workflowID
func findWorkflowIDByRunID(ctx context.Context, runID string) (string, error) {
	// List open workflows and search for the matching runID
	var nextPageToken []byte
	for {
		resp, err := temporalClient.ListOpenWorkflow(ctx, &workflowservice.ListOpenWorkflowExecutionsRequest{
			Namespace:       client.DefaultNamespace,
			MaximumPageSize: 100,
			NextPageToken:   nextPageToken,
		})
		if err != nil {
			return "", err
		}

		// Search through the results for matching runID
		for _, exec := range resp.Executions {
			if exec.Execution.RunId == runID {
				return exec.Execution.WorkflowId, nil
			}
		}

		// Check if there are more pages
		nextPageToken = resp.NextPageToken
		if len(nextPageToken) == 0 {
			break
		}
	}

	// If not found in open workflows, try closed workflows
	nextPageToken = nil
	for {
		resp, err := temporalClient.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Namespace:     client.DefaultNamespace,
			PageSize:      100,
			NextPageToken: nextPageToken,
		})
		if err != nil {
			return "", err
		}

		// Search through the results for matching runID
		for _, exec := range resp.Executions {
			if exec.Execution.RunId == runID {
				return exec.Execution.WorkflowId, nil
			}
		}

		// Check if there are more pages
		nextPageToken = resp.NextPageToken
		if len(nextPageToken) == 0 {
			break
		}
	}

	return "", nil // Not found
}

// GetWorkflowStatusResponse represents the response from getting workflow status
type GetWorkflowStatusResponse struct {
	WorkflowID     string              `json:"workflow_id"`
	RunID          string              `json:"run_id"`
	Status         string              `json:"status"`
	ProcessedSteps []ProcessedStepInfo `json:"processed_steps"`
	StartTime      *time.Time          `json:"start_time,omitempty"`
	CloseTime      *time.Time          `json:"close_time,omitempty"`
}

// ProcessedStepInfo represents information about a processed step
type ProcessedStepInfo struct {
	Step         string                 `json:"step"`
	Node         string                 `json:"node"`
	ActivityName string                 `json:"activity_name,omitempty"`
	EventType    string                 `json:"event_type,omitempty"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// getWorkflowStatusHandler handles GET requests to retrieve workflow status and processed steps
func getWorkflowStatusHandler(c echo.Context) error {
	workflowID := c.Param("workflow_id")
	if workflowID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "workflow_id is required",
		})
	}

	// Optional run_id query parameter
	runID := c.QueryParam("run_id")

	// Describe workflow execution to get status and memos
	ctx := context.Background()
	resp, err := temporalClient.DescribeWorkflowExecution(ctx, workflowID, runID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Unable to describe workflow: %v", err),
		})
	}

	workflowExecutionInfo := resp.GetWorkflowExecutionInfo()
	if workflowExecutionInfo == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("Workflow not found: %s", workflowID),
		})
	}

	// Get workflow status
	status := workflowExecutionInfo.GetStatus()
	statusString := getStatusString(status)

	// Extract processed steps from memos
	processedSteps := extractProcessedSteps(workflowExecutionInfo.GetMemo())

	// Get start and close times
	var startTime *time.Time
	var closeTime *time.Time

	if workflowExecutionInfo.GetStartTime() != nil {
		st := workflowExecutionInfo.GetStartTime().AsTime()
		startTime = &st
	}

	if workflowExecutionInfo.GetCloseTime() != nil {
		ct := workflowExecutionInfo.GetCloseTime().AsTime()
		closeTime = &ct
	}

	response := GetWorkflowStatusResponse{
		WorkflowID:     workflowExecutionInfo.GetExecution().GetWorkflowId(),
		RunID:          workflowExecutionInfo.GetExecution().GetRunId(),
		Status:         statusString,
		ProcessedSteps: processedSteps,
		StartTime:      startTime,
		CloseTime:      closeTime,
	}

	return c.JSON(http.StatusOK, response)
}

// getStatusString converts WorkflowExecutionStatus enum to string
func getStatusString(status enumspb.WorkflowExecutionStatus) string {
	switch status {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return "running"
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return "completed"
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED:
		return "failed"
	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return "canceled"
	case enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return "terminated"
	case enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return "timed_out"
	case enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return "continued_as_new"
	default:
		return "unknown"
	}
}

// extractProcessedSteps extracts processed steps from workflow memos
func extractProcessedSteps(memo *common.Memo) []ProcessedStepInfo {
	if memo == nil || len(memo.GetFields()) == 0 {
		return []ProcessedStepInfo{}
	}

	type stepWithTime struct {
		stepInfo  ProcessedStepInfo
		startedAt time.Time
	}

	var stepsWithTime []stepWithTime
	dataConverter := converter.GetDefaultDataConverter()

	// Iterate through memo fields to find step results
	// Step results are stored with keys like "activity_result_<step_name>"
	for key, payload := range memo.GetFields() {
		if len(key) > 16 && key[:16] == "activity_result_" {
			var stepMemo map[string]interface{}
			if err := dataConverter.FromPayload(payload, &stepMemo); err != nil {
				log.Printf("Warning: Failed to decode memo for key '%s': %v", key, err)
				continue
			}

			stepInfo := ProcessedStepInfo{}
			var startedAt time.Time
			hasStartedAt := false

			// Extract step name
			if step, ok := stepMemo["step"].(string); ok {
				stepInfo.Step = step
			}

			// Extract node name
			if node, ok := stepMemo["node"].(string); ok {
				stepInfo.Node = node
			}

			// Extract activity name
			if activityName, ok := stepMemo["activity_name"].(string); ok {
				stepInfo.ActivityName = activityName
			}

			// Extract event type
			if eventType, ok := stepMemo["event_type"].(string); ok {
				stepInfo.EventType = eventType
			}

			// Extract started_at (for sorting by execution order)
			if startedAtStr, ok := stepMemo["started_at"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
					startedAt = parsed
					hasStartedAt = true
					stepInfo.StartedAt = &parsed
				}
			} else if startedAtTime, ok := stepMemo["started_at"].(time.Time); ok {
				startedAt = startedAtTime
				hasStartedAt = true
				stepInfo.StartedAt = &startedAtTime
			}

			// Extract completed_at
			if completedAtStr, ok := stepMemo["completed_at"].(string); ok {
				if completedAt, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
					stepInfo.CompletedAt = &completedAt
					// If started_at is not available, use completed_at for sorting
					if !hasStartedAt {
						startedAt = completedAt
						hasStartedAt = true
					}
				}
			} else if completedAtTime, ok := stepMemo["completed_at"].(time.Time); ok {
				stepInfo.CompletedAt = &completedAtTime
				// If started_at is not available, use completed_at for sorting
				if !hasStartedAt {
					startedAt = completedAtTime
					hasStartedAt = true
				}
			}

			// Extract error
			if err, ok := stepMemo["error"].(string); ok {
				stepInfo.Error = err
			}

			// Extract metadata
			if metadata, ok := stepMemo["metadata"].(map[string]interface{}); ok {
				stepInfo.Metadata = metadata
			}

			// Only add steps that have a timestamp for sorting
			if hasStartedAt {
				stepsWithTime = append(stepsWithTime, stepWithTime{
					stepInfo:  stepInfo,
					startedAt: startedAt,
				})
			} else {
				// If no timestamp available, add with zero time (will be sorted first)
				stepsWithTime = append(stepsWithTime, stepWithTime{
					stepInfo:  stepInfo,
					startedAt: time.Time{},
				})
			}
		}
	}

	// Sort steps by started_at to maintain execution order
	sort.Slice(stepsWithTime, func(i, j int) bool {
		return stepsWithTime[i].startedAt.Before(stepsWithTime[j].startedAt)
	})

	// Extract just the stepInfo from the sorted slice
	steps := make([]ProcessedStepInfo, len(stepsWithTime))
	for i, swt := range stepsWithTime {
		steps[i] = swt.stepInfo
	}

	return steps
}

// NodeResponse represents the response structure for a single node
type NodeResponse struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"` // "activity" or "workflow_task"
	RetryPolicy *RetryPolicyResponse   `json:"retry_policy,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"` // JSON Schema format
}

// RetryPolicyResponse represents the retry policy information
type RetryPolicyResponse struct {
	InitialInterval    string  `json:"initial_interval,omitempty"` // Duration as string (e.g., "1s")
	BackoffCoefficient float64 `json:"backoff_coefficient,omitempty"`
	MaximumInterval    string  `json:"maximum_interval,omitempty"` // Duration as string
	MaximumAttempts    int32   `json:"maximum_attempts,omitempty"`
}

// GetNodesResponse represents the response from the get nodes endpoint
type GetNodesResponse struct {
	Nodes map[string]NodeResponse `json:"nodes"` // Key is node name, value is node info
}

// getNodesHandler handles GET requests to retrieve all available nodes
func getNodesHandler(c echo.Context) error {
	reg := register.GetInstance()
	nodeNames := reg.GetAllNodeNames()

	nodes := make(map[string]NodeResponse)

	for _, nodeName := range nodeNames {
		nodeInfo, exists := reg.GetNodeInfo(nodeName)
		if !exists {
			continue
		}

		nodeResponse := NodeResponse{
			Name: nodeInfo.Name,
		}

		// Set node type
		switch nodeInfo.Type {
		case register.NodeTypeActivity:
			nodeResponse.Type = "activity"
		case register.NodeTypeWorkflowTask:
			nodeResponse.Type = "workflow_task"
		}

		// Set retry policy if available
		if nodeInfo.RetryPolicy != nil {
			nodeResponse.RetryPolicy = &RetryPolicyResponse{
				InitialInterval:    nodeInfo.RetryPolicy.InitialInterval.String(),
				BackoffCoefficient: nodeInfo.RetryPolicy.BackoffCoefficient,
				MaximumInterval:    nodeInfo.RetryPolicy.MaximumInterval.String(),
				MaximumAttempts:    nodeInfo.RetryPolicy.MaximumAttempts,
			}
		}

		// Convert schema to JSON Schema format if available
		if nodeInfo.Schema != nil && nodeInfo.Schema.SchemaStruct != nil {
			jsonSchemaBytes, err := validation.ConvertStructToJSONSchema(nodeInfo.Schema.SchemaStruct)
			if err != nil {
				log.Printf("Warning: Failed to convert schema for node '%s': %v", nodeName, err)
			} else {
				var jsonSchema map[string]interface{}
				if err := json.Unmarshal(jsonSchemaBytes, &jsonSchema); err != nil {
					log.Printf("Warning: Failed to unmarshal JSON schema for node '%s': %v", nodeName, err)
				} else {
					nodeResponse.Schema = jsonSchema
				}
			}
		}

		nodes[nodeName] = nodeResponse
	}

	response := GetNodesResponse{
		Nodes: nodes,
	}

	return c.JSON(http.StatusOK, response)
}
