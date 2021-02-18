package station

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/figadore/go-intercom/pkg/call"
	"github.com/warthog618/gpiod"
)

type Station struct {
	CallManager call.Manager
	// Context     context.Context
	Inputs     Inputs
	Outputs    Outputs
	Speaker    *Speaker
	Microphone *Microphone
	Status     *Status
}

func (station *Station) UpdateStatus() {
	log.Println("Updating station status")
	if !station.hasCalls() {
		log.Println("Station has no calls")
		station.Status.Clear(StatusCallConnected)
		station.Status.Clear(StatusIncomingCall)
		station.Status.Clear(StatusOutgoingCall)
	}
}

// New creates a Station
// ctx is the main context from cmd
func New(ctx context.Context, dotEnv map[string]string, callManagerFactory func(*Station) call.Manager) *Station {
	// get access to leds, display, etc
	outputs := getOutputs(dotEnv)
	speaker := Speaker{
		AudioCh: make(chan []float32),
		done:    make(chan struct{}),
	}
	mic := Microphone{
		AudioCh: make(chan []float32),
		done:    make(chan struct{}),
	}
	station := Station{
		Speaker:    &speaker,
		Microphone: &mic,
		Outputs:    outputs,
	}
	status := Status{
		status:  StatusDefault,
		station: &station,
	}
	station.Status = &status
	callManager := callManagerFactory(&station)
	station.CallManager = callManager

	// get access to buttons, volume, etc
	inputs := getInputs(ctx, dotEnv, &station)
	station.Inputs = inputs
	return &station
}

func (s *Station) AcceptCall() {
	s.CallManager.AcceptCh() <- true
}

func (s *Station) RejectCall() {
	s.CallManager.AcceptCh() <- false
}

func (s *Station) callAll() {
	s.CallManager.CallAll()
}

func (s *Station) hangupAll() {
	s.CallManager.HangupAll()
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

// ctx is station context/main context from cmd
func getInputs(ctx context.Context, dotEnv map[string]string, station *Station) Inputs {
	if val, ok := dotEnv["INPUT_TYPE"]; ok && val == "button" {
		inputs := newPhysicalInputs(ctx, dotEnv, station)
		return inputs
	} else {
		panic(fmt.Sprintf("Unknown display type: %v", val))
	}
}

// StartPlayback begins the station's speaker playback
func (s *Station) StartPlayback(ctx context.Context, wg *sync.WaitGroup, errCh chan error) {
	// TODO create a new speaker here, or maybe just a new audio stream
	s.Speaker.done = make(chan struct{})
	s.Speaker.StartPlayback(ctx, wg, errCh)
}

// StartRecording begins the station's mic recording/listening
func (s *Station) StartRecording(ctx context.Context, wg *sync.WaitGroup, errCh chan error) {
	s.Microphone.done = make(chan struct{})
	s.Microphone.StartRecording(ctx, wg, errCh)
}

// SpeakerAudioCh returns the channel for sending speaker audio data
func (s *Station) SpeakerAudioCh() chan []float32 {
	return s.Speaker.AudioCh
}

// MicAudioCh returns the channel for receiving mic audio data
func (s *Station) MicAudioCh() chan []float32 {
	return s.Microphone.AudioCh
}

// Release resources for this device. Only do this on full shut down
func (s *Station) Close() {
	log.Println("Station.Close()")
	s.Inputs.Close()
	s.Outputs.Close()
	s.Speaker.Close()
	s.Microphone.Close()
	log.Println("Station.Closed()")
}

func (s *Station) hasCalls() bool {
	// m := *s.CallManager
	return s.CallManager.HasCalls()
}

func reserveChip() *gpiod.Chip {
	chip, err := gpiod.NewChip("gpiochip0")
	if err != nil {
		panic(err)
	}
	return chip
}
