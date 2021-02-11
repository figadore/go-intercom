package station

import (
	"context"
	"fmt"
	"sync"

	"github.com/figadore/go-intercom/pkg/call"
	"github.com/warthog618/gpiod"
)

type Status struct {
	sync.Mutex
	status int
}

func (s *Status) Set(flag int) int {
	s.Lock()
	defer s.Unlock()
	s.status = s.status | flag
	return s.status
}

func (s *Status) Has(flag int) bool {
	s.Lock()
	defer s.Unlock()
	return s.status&flag != 0
}

func (s *Status) Clear(flag int) int {
	s.Lock()
	defer s.Unlock()
	s.status = s.status &^ flag
	return s.status
}

func (s *Status) Toggle(flag int) int {
	s.Lock()
	defer s.Unlock()
	s.status = s.status ^ flag
	return s.status
}

type Station struct {
	sync.Mutex
	CallManager call.Manager
	Context     context.Context
	Inputs      Inputs
	Outputs     Outputs
	Speaker     *Speaker
	Microphone  *Microphone
	Status      Status
}

// Bitmask to handle multiple simultaneous states
const (
	statusUnknown = 1 << iota
	statusDefault
	statusDoNotDisturb
	statusIncomingCall
	statusOutgoingCall
	statusCallConnected
)

func New(ctx context.Context, dotEnv map[string]string, callManagerFactory func() call.Manager) *Station {
	// get access to leds, display, etc
	outputs := getOutputs(dotEnv)
	callManager := callManagerFactory()

	speaker := Speaker{
		AudioCh: make(chan []float32),
		Context: ctx,
	}
	mic := Microphone{
		AudioCh: make(chan []float32),
		Context: ctx,
	}
	// Initialize the station (getInputs needs it for the callback)
	station := Station{
		Context:     ctx,
		CallManager: callManager,
		Outputs:     outputs,
		Speaker:     &speaker,
		Microphone:  &mic,
		Status:      Status{status: statusUnknown},
	}

	// get access to buttons, volume, etc
	inputs := getInputs(ctx, dotEnv, &station)
	station.Inputs = inputs
	return &station
}

func (s *Station) StartPlayback(ctx context.Context, errCh chan error) {
	s.Speaker.StartPlayback(ctx, errCh)
}

func (s *Station) StartRecording(ctx context.Context, errCh chan error) {
	s.Microphone.StartRecording(ctx, errCh)
}

func (s *Station) SendSpeakerAudio(data []float32) {
	s.Speaker.AudioCh <- data
}

func (s *Station) ReceiveMicAudio() []float32 {
	return <-s.Microphone.AudioCh
}

func (s *Station) Close() {
	s.Inputs.Close()
	s.Outputs.Close()
	s.Speaker.Close()
	s.Microphone.Close()
}

func (s *Station) hasCalls() bool {
	// m := *s.CallManager
	return s.CallManager.HasCalls()
}

func (s *Station) callAll(ctx context.Context) {
	s.CallManager.CallAll(ctx)
}

func (s *Station) hangup() {
	s.CallManager.Hangup()
}

func reserveChip() *gpiod.Chip {
	chip, err := gpiod.NewChip("gpiochip0")
	if err != nil {
		panic(err)
	}
	return chip
}

func getOutputs(dotEnv map[string]string) Outputs {
	if val, ok := dotEnv["OUTPUT_TYPE"]; ok && val == "led" {
		chip := reserveChip()
		defer chip.Close()
		return newLedDisplay(chip)
	} else {
		panic(fmt.Sprintf("Unknown output type: %v", val))
	}
}

func getInputs(ctx context.Context, dotEnv map[string]string, station *Station) Inputs {
	if val, ok := dotEnv["INPUT_TYPE"]; ok && val == "button" {
		inputs := newPhysicalInputs(ctx, dotEnv, station)
		return inputs
	} else {
		panic(fmt.Sprintf("Unknown display type: %v", val))
	}
}
