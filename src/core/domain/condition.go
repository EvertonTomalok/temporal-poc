package domain

// StepInput defines the input structure for a step
type StepInput struct {
	TimeoutSeconds int64 `json:"timeout_seconds,omitempty"` // Timeout in seconds (optional)
}

// Condition defines conditional branching with condition evaluation and multiple outcomes
type Condition struct {
	Condition    string `json:"condition,omitempty"`     // The condition to evaluate
	Description  string `json:"description,omitempty"`   // Description of the condition
	Satisfied    string `json:"satisfied,omitempty"`     // Next step when condition is satisfied (e.g., "step-3")
	NotSatisfied string `json:"not_satisfied,omitempty"` // Next step when condition is not satisfied (e.g., "step-4")
	Timeout      string `json:"timeout,omitempty"`       // Next step when timeout event occurs (e.g., "step-5")
}

func (c *Condition) GetNextStep(eventType EventType) string {
	switch eventType {
	case EventTypeConditionSatisfied:
		return c.Satisfied
	case EventTypeConditionNotSatisfied:
		return c.NotSatisfied
	case EventTypeConditionTimeout:
		return c.Timeout
	default:
		return ""
	}
}
