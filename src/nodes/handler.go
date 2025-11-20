package nodes

import (
	"time"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

// HandlerContext holds the state and context for handlers
type HandlerContext struct {
	ClientAnswered  bool
	StartTime       time.Time
	TimeoutDuration time.Duration
	Logger          log.Logger
	Timer           workflow.Future
	CancelTimer     workflow.CancelFunc
}

// HandlerResult indicates whether the workflow should continue or stop
type HandlerResult int

const (
	Continue HandlerResult = iota
	Stop
)

// SignalHandler defines the interface for chain of responsibility nodes
type SignalHandler interface {
	// Handle processes the signal and returns whether to continue or stop
	Handle(ctx workflow.Context, handlerCtx *HandlerContext, selector workflow.Selector) HandlerResult
	// SetNext sets the next handler in the chain
	SetNext(handler SignalHandler)
}

// HandlerChain manages the chain of responsibility
type HandlerChain struct {
	firstHandler SignalHandler
}

// NewHandlerChain creates a new handler chain
func NewHandlerChain() *HandlerChain {
	return &HandlerChain{}
}

// AddHandler adds a handler to the chain
func (hc *HandlerChain) AddHandler(handler SignalHandler) {
	if hc.firstHandler == nil {
		hc.firstHandler = handler
		return
	}

	// Find the last handler and set next
	current := hc.firstHandler
	for {
		if next := getNextHandler(current); next == nil {
			current.SetNext(handler)
			break
		} else {
			current = next
		}
	}
}

// Process runs the chain of handlers
func (hc *HandlerChain) Process(ctx workflow.Context, handlerCtx *HandlerContext, selector workflow.Selector) HandlerResult {
	if hc.firstHandler == nil {
		return Stop
	}
	return hc.firstHandler.Handle(ctx, handlerCtx, selector)
}

// BaseHandler provides common functionality for handlers
type BaseHandler struct {
	next SignalHandler
}

func (h *BaseHandler) SetNext(handler SignalHandler) {
	h.next = handler
}

func (h *BaseHandler) GetNext() SignalHandler {
	return h.next
}

func getNextHandler(handler SignalHandler) SignalHandler {
	// Access next handler through BaseHandler
	// All handlers embed BaseHandler, so we can access it via GetNext method
	if baseHandler, ok := handler.(interface{ GetNext() SignalHandler }); ok {
		return baseHandler.GetNext()
	}
	return nil
}
