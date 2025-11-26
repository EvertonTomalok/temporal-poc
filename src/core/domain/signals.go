package domain

const (
	ClientAnsweredSignal = "client-answered"
	StopSignal           = "stop"
)

// ClientAnsweredSignalPayload represents the payload sent with the client-answered signal
type ClientAnsweredSignalPayload struct {
	Message string `json:"message"`
}
