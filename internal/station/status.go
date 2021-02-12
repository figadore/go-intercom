package station

import (
	"sync"
)

type Status struct {
	sync.Mutex
	status  int
	station *Station
}

func (s *Status) Has(flag int) bool {
	s.Lock()
	defer s.Unlock()
	return s.status&flag != 0
}

func (s *Status) Set(flag int) int {
	s.Lock()
	s.status = s.status | flag
	s.Unlock()
	s.station.Outputs.UpdateStatus(s)
	return s.status
}

func (s *Status) Clear(flag int) int {
	s.Lock()
	s.status = s.status &^ flag
	s.Unlock()
	s.station.Outputs.UpdateStatus(s)
	return s.status
}

func (s *Status) Toggle(flag int) int {
	s.Lock()
	s.status = s.status ^ flag
	s.Unlock()
	s.station.Outputs.UpdateStatus(s)
	return s.status
}

// Bitmask to handle multiple simultaneous states
const (
	StatusDefault = 0
	StatusError   = 1 << iota
	StatusDoNotDisturb
	StatusIncomingCall
	StatusOutgoingCall
	StatusCallConnected
)
