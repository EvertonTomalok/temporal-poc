package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"temporal-poc/src/core"
	"temporal-poc/src/nodes"
	workflows "temporal-poc/src/workflows"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.temporal.io/sdk/client"
)

var temporalClient client.Client

func init() {
	// Create Temporal client on startup
	var err error
	temporalClient, err = client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
	})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}

	// Register search attributes if they don't exist
	// This MUST succeed before the server can handle workflow requests
	log.Println("Checking and registering search attributes...")
	if err := nodes.RegisterSearchAttributesIfNeeded(temporalClient); err != nil {
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
	e.POST("/start-workflow", startWorkflowHandler)
	e.POST("/send-signal", sendSignalHandler)

	// Start server
	port := ":8081"
	log.Printf("Server starting on port %s\n", port)
	log.Println("Endpoints:")
	log.Println("  POST /start-workflow - Start a new workflow")
	log.Println("  POST /send-signal - Send a signal to a workflow")

	if err := e.Start(port); err != nil && err != http.ErrServerClosed {
		log.Fatalln("Server failed to start", err)
	}
}

// StartWorkflowRequest represents the request body for starting a workflow
type StartWorkflowRequest struct {
	// Optional: if not provided, a UUID will be generated
	WorkflowID string `json:"workflow_id,omitempty"`
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
		workflowID = "abandonedCart-" + uuid.New().String()
	}

	// Start workflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: core.PrimaryWorkflowTaskQueue,
	}

	we, err := temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, workflows.AbandonedCartWorkflow)
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

	// Default signal name to "client-answered" if not provided
	signalName := req.SignalName
	if signalName == "" {
		signalName = core.ClientAnsweredSignal
	}

	// Send signal to workflow
	// Use empty RunID to signal the latest run if not provided
	runID := req.RunID
	err := temporalClient.SignalWorkflow(context.Background(), req.WorkflowID, runID, signalName, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Unable to signal workflow: %v", err),
		})
	}

	// Return response
	response := SendSignalResponse{
		Message: fmt.Sprintf("Successfully sent '%s' signal to workflow: %s", signalName, req.WorkflowID),
	}

	return c.JSON(http.StatusOK, response)
}
