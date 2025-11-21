package domain

const (
	ClientAnsweredSignal = "client-answered"
)

// ClientAnsweredSignalPayload represents the payload sent with the client-answered signal
type ClientAnsweredSignalPayload struct {
	Message string `json:"message"`
}
