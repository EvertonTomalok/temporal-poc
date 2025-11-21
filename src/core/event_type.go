package core

type EventType string

func (e EventType) String() string {
	return string(e)
}

const (
	EventTypeSatisfied    EventType = "satisfied"
	EventTypeNotSatisfied EventType = "not_satisfied"
	EventTypeTimeout      EventType = "timeout"
)
