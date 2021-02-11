package call

import (
	"context"

	"github.com/rs/xid"
)

const (
	// Bitmask to handle multiple simultaneous states
	CallStatusPending = 1 << iota
	CallStatusActive
	CallStatusTerminating
)

type ContextKey string

type Call struct {
	Id     xid.ID
	To     string
	From   string
	Status int
	Cancel func()
}

//type Caller interface {
//	call(string)
//	Hangup(string)
//}

func (c *Call) Hangup() {
	c.Status = CallStatusTerminating
	c.Cancel()
	// TODO remove from call list when terminated
}

type Manager interface {
	CallAll(context.Context)
	Hangup()
	HasCalls() bool
	// PlaceCall(ctx context.Context, to []string)
	// ServeCall(ctx context.Context, from string)
}

type GenericManager struct {
	callList []Call
}

func (m *GenericManager) HasCalls() bool {
	for _, call := range m.callList {
		if call.Status|CallStatusPending|CallStatusActive != 0 {
			return true
		}
	}
	return false
}
