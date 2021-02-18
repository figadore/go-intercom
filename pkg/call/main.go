package call

import (
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

type CallId xid.ID

func (id CallId) String() string {
	return xid.ID(id).String()
}

func NewCallId() CallId {
	return CallId(xid.New())
}

type Call struct {
	Id     CallId
	To     string
	From   string
	Status Status
	cancel func()
	// TODO add pointer to call manager? or at least a callback when when cancel is called?
}

func New(callId CallId, to string, from string, cancel func()) *Call {
	call := Call{
		Id:     callId,
		To:     to,
		From:   from,
		cancel: cancel,
		Status: StatusPending,
	}
	return &call
}

//type Caller interface {
//	call(string)
//	Hangup(string)
//}

func (c *Call) Hangup() {
	c.Status = StatusTerminating
	c.cancel()
}

type Manager interface {
	CallAll()
	HangupAll()
	AcceptCall()
	RejectCall()
	AcceptCh() chan bool
	HasCalls() bool
	// PlaceCall(ctx context.Context, to []string)
	// ServeCall(ctx context.Context, from string)
}

type GenericManager struct {
	CallList map[CallId]*Call
}

func (m *GenericManager) HasCalls() bool {
	for _, call := range m.CallList {
		if (call.Status&StatusPending)|(call.Status&StatusActive) != 0 {
			return true
		}
	}
	return false
}
