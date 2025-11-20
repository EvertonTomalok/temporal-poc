package core

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Search attribute keys for client-answered tracking
var (
	// ClientAnsweredField is a boolean search attribute indicating if client has answered
	ClientAnsweredField = temporal.NewSearchAttributeKeyBool("ClientAnswered")
	// ClientAnsweredAtField is a datetime search attribute for when client answered
	ClientAnsweredAtField = temporal.NewSearchAttributeKeyTime("ClientAnsweredAt")
)

// RegisterSearchAttributesIfNeeded registers the required search attributes with the Temporal server
// if they don't already exist. This should be called during worker initialization.
func RegisterSearchAttributesIfNeeded(c client.Client) error {
	// Get the host port from the client options or use default
	hostPort := client.DefaultHostPort

	// Try to extract host port from client using type assertion
	// The client interface doesn't expose this directly, so we'll use the default
	// or try to get it from the client's internal state

	// Create a new gRPC connection for the operator service
	// Use insecure credentials for local development (use TLS in production)
	log.Printf("Connecting to Temporal server at %s to register search attributes...", hostPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, hostPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("unable to create gRPC connection to %s: %w. Please register search attributes manually", hostPort, err)
	}
	defer conn.Close()
	log.Println("Connected to Temporal server successfully")

	operatorClient := operatorservice.NewOperatorServiceClient(conn)

	// Define the search attributes we need
	searchAttributes := map[string]enums.IndexedValueType{
		"ClientAnswered":   enums.INDEXED_VALUE_TYPE_BOOL,
		"ClientAnsweredAt": enums.INDEXED_VALUE_TYPE_DATETIME,
	}

	// First, list existing search attributes to check what's already registered
	log.Println("Listing existing search attributes...")
	listReq := &operatorservice.ListSearchAttributesRequest{}
	listResp, err := operatorClient.ListSearchAttributes(ctx, listReq)
	if err != nil {
		return fmt.Errorf("failed to list search attributes (check connection and permissions): %w", err)
	}

	log.Printf("Found %d custom search attributes already registered", len(listResp.CustomAttributes))

	// Check which attributes need to be added
	attributesToAdd := make(map[string]enums.IndexedValueType)
	for name, valueType := range searchAttributes {
		if _, exists := listResp.CustomAttributes[name]; !exists {
			attributesToAdd[name] = valueType
		} else {
			log.Printf("Search attribute '%s' already exists, skipping registration", name)
		}
	}

	// If all attributes already exist, we're done
	if len(attributesToAdd) == 0 {
		log.Println("All required search attributes are already registered")
		return nil
	}

	// Add the missing search attributes
	addReq := &operatorservice.AddSearchAttributesRequest{
		SearchAttributes: attributesToAdd,
	}

	_, err = operatorClient.AddSearchAttributes(ctx, addReq)
	if err != nil {
		// Check if it's an "already exists" error (which is fine)
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") ||
			strings.Contains(errMsg, "AlreadyExists") ||
			strings.Contains(errMsg, "already registered") {
			log.Println("Search attributes already exist (race condition handled)")
			return nil
		}
		return fmt.Errorf("failed to add search attributes: %w", err)
	}

	log.Printf("Successfully registered %d search attribute(s)", len(attributesToAdd))
	for name := range attributesToAdd {
		log.Printf("  - %s", name)
	}

	// Wait a moment for the server to process the registration
	// This helps ensure the attributes are available before workflows start
	time.Sleep(500 * time.Millisecond)

	// Verify the registration by listing again
	listResp2, err := operatorClient.ListSearchAttributes(ctx, &operatorservice.ListSearchAttributesRequest{})
	if err == nil {
		allRegistered := true
		for name := range attributesToAdd {
			if _, exists := listResp2.CustomAttributes[name]; !exists {
				log.Printf("Warning: Search attribute '%s' was registered but not found in verification", name)
				allRegistered = false
			}
		}
		if allRegistered {
			log.Println("Verified: All search attributes are now registered")
		}
	}

	return nil
}
