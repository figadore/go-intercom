package call

import (
	"context"

	"github.com/rs/xid"
)

type Status int

const (
	// Bitmask to handle multiple simultaneous states
	StatusPending = Status(1 << iota)
	StatusActive
	StatusTerminating
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusActive:
		return "Active"
	case StatusTerminating:
		return "Terminating"
	}
	return "Unknown"
}

type ContextKey string

type Call struct {
	Id     xid.ID
	To     string
	From   string
	Status Status
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
	AcceptCall()
	RejectCall()
	AcceptCh() chan bool
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
