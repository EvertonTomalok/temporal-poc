package core

type EventType string

func (e EventType) String() string {
	return string(e)
}

const (
	EventTypeSuccess EventType = "success"
	EventTypeTimeout EventType = "timeout"
)
