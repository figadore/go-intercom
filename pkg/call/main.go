package call

import (
	"context"
)

const (
	// Bitmask to handle multiple simultaneous states
	CallStatusPending = 1 << iota
	CallStatusActive
	CallStatusTerminating
)

type ContextKey string

type Call struct {
	To     string
	From   string
	Status int
	Cancel func()
}

type Caller interface {
	call(string)
	Hangup(string)
}

func (c *Call) Hangup() {
	c.Status = CallStatusTerminating
	c.Cancel()
	// TODO remove from call list when terminated
}

type Manager interface {
	CallAll(context.Context)
	EndCalls()
}
