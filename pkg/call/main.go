package call

import (
	"context"
)

const (
	// Bitmask to handle multiple simultaneous states
	callStatusPending = 1 << iota
	callStatusOngoing
)

type Call struct {
	to     string
	from   string
	status int
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
