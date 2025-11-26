package domain

const (
	ClientAnsweredSignal = "client-answered"
	StopSignal           = "stop"
)

// ClientAnsweredSignalPayload represents the payload sent with the client-answered signal
type ClientAnsweredSignalPayload struct {
	Message  string                 `json:"message"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// StopSignalPayload represents the payload sent with the stop signal
type StopSignalPayload struct {
	ConditionStatus string `json:"condition_status"` // Required: must be "satisfied" or "not_satisfied"
}
