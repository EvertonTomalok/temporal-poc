package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// StartRecoveryCartRequest represents the request payload for the recovery cart API
type StartRecoveryCartRequest struct {
	WorkflowID string `json:"workflow_id"`
	UserID     string `json:"user_id,omitempty"`
	CreatorId  string `json:"creator_id,omitempty"`
	OfferID    string `json:"offer_id,omitempty"`
	CartID     string `json:"cart_id,omitempty"`
}

// StartRecoveryCartResponse represents the response from the recovery cart API
type StartRecoveryCartResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// LeadService provides methods for lead-related operations
type LeadService interface {
	StartRecoveryCart(ctx context.Context, req StartRecoveryCartRequest) (*StartRecoveryCartResponse, error)
}

// leadService implements LeadService
type leadService struct{}

// NewLeadService creates a new LeadService instance
func NewLeadService() LeadService {
	return &leadService{}
}

// StartRecoveryCart simulates a call to an internal API for starting a recovery cart
// This is a mock implementation that simulates network latency and API response
func (s *leadService) StartRecoveryCart(ctx context.Context, req StartRecoveryCartRequest) (*StartRecoveryCartResponse, error) {
	// Simulate API call latency (1-3 seconds)
	latency := time.Duration(rand.Intn(1000)+300) * time.Millisecond

	// Check if context is cancelled before sleeping
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		time.Sleep(latency)
	}

	// Simulate API response - 90% success rate, 10% failure rate
	randomValue := rand.Intn(101)
	success := randomValue < 90

	if success {
		return &StartRecoveryCartResponse{
			Success:   true,
			Message:   "Recovery cart started successfully",
			Timestamp: time.Now(),
		}, nil
	}

	// Simulate API error
	return nil, fmt.Errorf("internal API error: failed to start recovery cart (simulated failure)")
}

// StartRecoveryCart is a convenience function that uses the default lead service
// DEPRECATED: Use LeadService interface instead
func StartRecoveryCart(ctx context.Context, req StartRecoveryCartRequest) (*StartRecoveryCartResponse, error) {
	service := NewLeadService()
	return service.StartRecoveryCart(ctx, req)
}
