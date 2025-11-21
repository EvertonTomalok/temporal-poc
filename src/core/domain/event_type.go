package domain

type EventType string

func (e EventType) String() string {
	return string(e)
}

const (
	EventTypeConditionSatisfied    EventType = "condition_satisfied"
	EventTypeConditionNotSatisfied EventType = "condition_not_satisfied"
	EventTypeConditionTimeout      EventType = "condition_timeout"
)
