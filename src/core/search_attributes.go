package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Search attribute keys for client-answered tracking
var (
	ClientAnsweredField   = temporal.NewSearchAttributeKeyBool("ClientAnswered")
	ClientAnsweredAtField = temporal.NewSearchAttributeKeyTime("ClientAnsweredAt")
)

// SearchAttributeKey is an interface for search attribute keys
type SearchAttributeKey interface {
	GetName() string
	GetValueType() enums.IndexedValueType
}

// SearchAttributeKeys is an enum-like slice of all search attribute keys
var SearchAttributeKeys = []SearchAttributeKey{
	ClientAnsweredField,
	ClientAnsweredAtField,
}

// RegisterSearchAttributesIfNeeded registers the required search attributes with the Temporal server
// if they don't already exist. This should be called during worker initialization.
// It accepts clientOptions to use the same connection settings (host, TLS, credentials) as the client.
func RegisterSearchAttributesIfNeeded(c client.Client, clientOptions client.Options) error {
	// Get the host port from the client options
	hostPort := clientOptions.HostPort
	if hostPort == "" {
		hostPort = client.DefaultHostPort
	}

	// Determine if we need TLS (for Temporal Cloud)
	useTLS := clientOptions.Credentials != nil || clientOptions.ConnectionOptions.TLS != nil

	// Create a new gRPC connection for the operator service
	// Use the same connection settings as the client (TLS and credentials for cloud)
	log.Printf("Connecting to Temporal server at %s to register search attributes...", hostPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var dialOptions []grpc.DialOption

	if useTLS {
		// Use TLS credentials for Temporal Cloud
		tlsConfig := clientOptions.ConnectionOptions.TLS
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		// Use insecure credentials for local development
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, hostPort, dialOptions...)
	if err != nil {
		return fmt.Errorf("unable to create gRPC connection to %s: %w. Please register search attributes manually", hostPort, err)
	}
	defer conn.Close()
	log.Println("Connected to Temporal server successfully")

	operatorClient := operatorservice.NewOperatorServiceClient(conn)

	// Extract API key from credentials and add to metadata if available
	// For Temporal Cloud, the API key needs to be sent in the authorization header
	var apiKey string
	if clientOptions.Credentials != nil {
		// Try to extract API key from credentials using reflection or read from env
		// The Temporal SDK's APIKeyStaticCredentials doesn't expose the key directly,
		// so we read it from the environment variable as a fallback
		apiKey = os.Getenv("TEMPORAL_API_KEY")
	}

	// Create a context with metadata if we have an API key
	requestCtx := ctx
	if apiKey != "" {
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + apiKey,
		})
		requestCtx = metadata.NewOutgoingContext(ctx, md)
	}

	// First, list existing search attributes to check what's already registered
	log.Println("Listing existing search attributes...")
	listReq := &operatorservice.ListSearchAttributesRequest{}
	listResp, err := operatorClient.ListSearchAttributes(requestCtx, listReq)
	if err != nil {
		return fmt.Errorf("failed to list search attributes (check connection and permissions): %w", err)
	}

	log.Printf("Found %d custom search attributes already registered", len(listResp.CustomAttributes))

	// Check which attributes need to be added - iterate over the enum directly
	attributesToAdd := make(map[string]enums.IndexedValueType)
	for _, key := range SearchAttributeKeys {
		name := key.GetName()
		valueType := key.GetValueType()
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

	_, err = operatorClient.AddSearchAttributes(requestCtx, addReq)
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
	listResp2, err := operatorClient.ListSearchAttributes(requestCtx, &operatorservice.ListSearchAttributesRequest{})
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
