package station

import (
	"context"
	"fmt"

	"github.com/figadore/go-intercom/pkg/call"
	"github.com/warthog618/gpiod"
)

type Station struct {
	Inputs     Inputs
	Outputs    Outputs
	Speaker    Speaker
	Microphone Microphone
}

// Bitmask to handle multiple simultaneous states
const (
	statusDefault = 1 << iota
	statusDoNotDisturb
	statusIncomingCall
	statusOutgoingCall
	statusCallConnected
)

// Allow various ways to display status and other info
// E.g. LEDs, TFT, text to speech
type Outputs interface {
	IncomingCall(ctx context.Context, from string) error
	UpdateStatus(status int) error
	OutgoingCall(ctx context.Context, to string) error
	Close()
}

// Allow various ways to interact with the intercom
// E.g. buttons, menu with display, voice commands
type Inputs interface {
	acceptCall(ctx context.Context, from string, callManager call.Manager)
	placeCall(ctx context.Context, to []string, callManager call.Manager)
	callAll(ctx context.Context, callManager call.Manager)
	hangup(callManager call.Manager)
	setVolume(percent int)
	setDoNotDisturb(status int)
	Close()
}

func New(ctx context.Context, dotEnv map[string]string, callManager call.Manager) *Station {
	chip := reserveChip()
	defer chip.Close()
	// get access to leds, display, etc
	outputs := getOutputs(dotEnv)
	// get access to buttons, volume, etc
	inputs := getInputs(ctx, dotEnv, callManager)
	return &Station{
		Inputs:     inputs,
		Outputs:    outputs,
		Speaker:    Speaker{},
		Microphone: Microphone{},
	}
}

func (d Station) Close() {
	d.Inputs.Close()
	d.Outputs.Close()
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

func getInputs(ctx context.Context, dotEnv map[string]string, callManager call.Manager) Inputs {
	if val, ok := dotEnv["INPUT_TYPE"]; ok && val == "button" {
		chip := reserveChip()
		defer chip.Close()
		inputs := newPhysicalInputs(ctx, chip, dotEnv, callManager)
		return inputs
	} else {
		panic(fmt.Sprintf("Unknown display type: %v", val))
	}
}
