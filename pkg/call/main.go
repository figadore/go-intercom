package call

import (
	"context"
)

const (
	// Bitmask to handle multiple simultaneous states
	CallStatusPending = 1 << iota
	CallStatusOngoing
)

type Call struct {
	To     string
	From   string
	Status int
}

type Caller interface {
	call(string)
	Hangup(string)
}

func (*Call) Hangup() {
}

type Manager interface {
	CallAll(context.Context)
	EndCalls()
}
