package config

import (
	"crypto/tls"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
)

// LoadTemporalClientOptions loads Temporal client configuration from environment variables
// It loads from .env file if it exists, then reads from environment variables
// Returns client.Options configured for either local development or Temporal Cloud
// Additional options like Identity and Logger can be set after calling this function
func LoadTemporalClientOptions() client.Options {
	// Try to load .env file from multiple locations
	// First try current directory, then try project root (assuming we're in cmd/worker or cmd/server)
	envLoaded := false
	envPaths := []string{
		".env",                            // Current directory
		"../.env",                         // One level up (from cmd/worker or cmd/server)
		"../../.env",                      // Two levels up (from cmd/worker/main.go)
		filepath.Join("..", "..", ".env"), // Alternative path format
	}

	for _, envPath := range envPaths {
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("Loaded .env file from: %s", envPath)
			envLoaded = true
			break
		}
	}

	if !envLoaded {
		// .env file is optional, so we only log if there's an actual error (not just file not found)
		log.Println("No .env file found, using environment variables or defaults")
	}

	// Load configuration from environment variables or use defaults
	hostPort := os.Getenv("TEMPORAL_HOST_PORT")
	if hostPort == "" {
		hostPort = client.DefaultHostPort // Default to localhost:7233
	}

	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	apiKey := os.Getenv("TEMPORAL_API_KEY")

	// Log loaded configuration (without sensitive data)
	log.Printf("Temporal client config - HostPort: %s, Namespace: %s, HasAPIKey: %v",
		hostPort, namespace, apiKey != "")

	// Build client options
	clientOptions := client.Options{
		HostPort: hostPort,
	}

	// Configure namespace if provided
	if namespace != "" {
		clientOptions.Namespace = namespace
	}

	// Configure TLS and API key credentials if API key is provided (for Temporal Cloud)
	if apiKey != "" {
		clientOptions.ConnectionOptions = client.ConnectionOptions{
			TLS: &tls.Config{},
		}
		clientOptions.Credentials = client.NewAPIKeyStaticCredentials(apiKey)
	}

	return clientOptions
}
