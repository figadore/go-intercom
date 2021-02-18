package station

import (
	"sync"

	"github.com/figadore/go-intercom/internal/log"
)

type Status struct {
	sync.Mutex
	status  status
	station *Station
}

func (s *Status) Has(flag status) bool {
	s.Lock()
	defer s.Unlock()
	return s.status&flag != 0
}

func (s *Status) Set(flag status) status {
	log.Println("Setting status: ", flag)
	s.Lock()
	s.status = s.status | flag
	s.Unlock()
	s.station.Outputs.UpdateStatus(s)
	return s.status
}

func (s *Status) Clear(flag status) status {
	log.Println("Clearing status: ", flag)
	s.Lock()
	s.status = s.status &^ flag
	s.Unlock()
	s.station.Outputs.UpdateStatus(s)
	return s.status
}

func (s *Status) Toggle(flag status) status {
	log.Println("Toggling status: ", flag)
	s.Lock()
	s.status = s.status ^ flag
	s.Unlock()
	s.station.Outputs.UpdateStatus(s)
	return s.status
}

type status int

// Bitmask to handle multiple simultaneous states
const (
	StatusError         = status(1 << iota) // 1
	StatusDoNotDisturb                      // 2
	StatusIncomingCall                      // 4
	StatusOutgoingCall                      // 8
	StatusCallConnected                     // 16
	StatusDefault       = status(0)
)

func (s status) String() string {
	switch s {
	case StatusError:
		return "Error"
	case StatusDoNotDisturb:
		return "DoNotDisturb"
	case StatusIncomingCall:
		return "IncomingCall"
	case StatusOutgoingCall:
		return "OutgoingCall"
	case StatusCallConnected:
		return "CallConnected"
	case StatusDefault:
		return "Default"
	}
	return "Unknown"
}
