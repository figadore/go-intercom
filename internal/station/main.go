package station

import (
	"context"
	"fmt"
	"log"

	"github.com/figadore/go-intercom/pkg/call"
	"github.com/warthog618/gpiod"
)

type Station struct {
	CallManager call.Manager
	Context     context.Context
	Inputs      Inputs
	Outputs     Outputs
	Speaker     *Speaker
	Microphone  *Microphone
	Status      *Status
}

func (station *Station) UpdateStatus() {
	log.Println("Updating station status")
	if !station.hasCalls() {
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
		Context: ctx,
	}
	mic := Microphone{
		AudioCh: make(chan []float32),
		Context: ctx,
	}
	station := Station{
		Context:    ctx,
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

func (s *Station) callAll(mainContext context.Context) {
	s.CallManager.CallAll(mainContext)
}

func (s *Station) hangup() {
	s.CallManager.Hangup()
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
func (s *Station) StartPlayback(errCh chan error) {
	s.Speaker.StartPlayback(errCh)
}

// StartRecording begins the station's mic recording/listening
func (s *Station) StartRecording(ctx context.Context, errCh chan error) {
	s.Microphone.StartRecording(ctx, errCh)
}

// SendSpeakerAudio takes data and sends it to the speaker audio channel (blocking)
func (s *Station) SendSpeakerAudio(data []float32) {
	s.Speaker.AudioCh <- data
}

// SendSpeakerAudio returns data from the mic audio (blocking)
func (s *Station) ReceiveMicAudio() []float32 {
	return <-s.Microphone.AudioCh
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
