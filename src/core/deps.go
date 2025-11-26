package core

import "temporal-poc/src/services"

// Deps provides dependencies for activities
// All dependencies must be initialized in cmd/worker
type Deps struct {
	LeadService services.LeadService
}

// NewDeps creates a new Deps instance with the provided LeadService
func NewDeps(leadService services.LeadService) *Deps {
	return &Deps{
		LeadService: leadService,
	}
}
