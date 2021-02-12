package call

import (
	"context"

	"github.com/rs/xid"
)

const (
	// Bitmask to handle multiple simultaneous states
	StatusPending = 1 << iota
	StatusActive
	StatusTerminating
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
	c.Status = StatusTerminating
	c.Cancel()
}

type Manager interface {
	CallAll(context.Context)
	Hangup()
	HasCalls() bool
	// PlaceCall(ctx context.Context, to []string)
	// ServeCall(ctx context.Context, from string)
}

type GenericManager struct {
	CallList map[xid.ID]Call
}

func (m *GenericManager) HasCalls() bool {
	for _, call := range m.CallList {
		if (call.Status&StatusPending)|(call.Status&StatusActive) != 0 {
			return true
		}
	}
	return false
}
