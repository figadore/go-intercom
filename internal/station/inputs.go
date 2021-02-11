package station

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/warthog618/gpiod"
)

// Allow various ways to interact with the intercom
// E.g. buttons, menu with display, voice commands
type Inputs interface {
	acceptCall()
	placeCall(to []string)
	callAll()
	hangup()
	setVolume(percent int)
	setDoNotDisturb(bool)
	Close()
}

// E.g. buttons, stateful menu with display, voice commands
type physicalInputs struct {
	station                        *Station
	ctx                            context.Context
	groupCallButton, endCallButton *gpiod.Line
	// volumeControl                  *struct{}
}

// map of button handlers, take gpiod.LineEvent input
// TODO or should this be a struct?
// also, what creates it? it should be wrappers around handlers in calls/, like callAll, and endCall, (and set dnd?)
type Handlers map[string]func(gpiod.LineEvent)

func newPhysicalInputs(ctx context.Context, dotEnv map[string]string, station *Station) *physicalInputs {
	chip := reserveChip()
	defer chip.Close()
	// TODO intercom.Close hangs on the client side when context cancelled, find a way to allow it to close
	redButtonPin, _ := strconv.Atoi(dotEnv["RED_BUTTON_PIN"])
	log.Printf("Found pin %d for red button in .env ...\n", redButtonPin)
	blackButtonPin, _ := strconv.Atoi(dotEnv["BLACK_BUTTON_PIN"])
	log.Printf("Found pin %d for black button in .env ...\n", blackButtonPin)
	// Set up button lines
	inputs := &physicalInputs{
		station: station,
		ctx:     ctx,
	}
	groupCallButton, err := chip.RequestLine(blackButtonPin,
		gpiod.WithDebounce(time.Millisecond*30),
		gpiod.WithFallingEdge, // use WithBothEdges and a timer if long-press required
		gpiod.WithEventHandler(inputs.blackButtonHandler))
	if err != nil {
		msg := fmt.Sprintf("RequestLine returned error: %s\n", err)
		panic(msg)
	}
	endCallButton, err := chip.RequestLine(redButtonPin,
		gpiod.WithDebounce(time.Millisecond*30),
		gpiod.WithFallingEdge,
		gpiod.WithEventHandler(inputs.redButtonHandler))
	if err != nil {
		msg := fmt.Sprintf("RequestLine returned error: %s\n", err)
		log.Println(msg)
		panic(msg)
	}
	inputs.endCallButton = endCallButton
	inputs.groupCallButton = groupCallButton
	return inputs
}

func (i *physicalInputs) blackButtonHandler(gpiod.LineEvent) {
	log.Debugln("group call handler: callAll")
	defer log.Debugln("group call handler: completed callAll")
	if i.station.Status.Has(statusDoNotDisturb) && i.station.Status.Has(statusIncomingCall) {
		i.acceptCall()
	} else {
		i.callAll()
	}
}

func (i *physicalInputs) redButtonHandler(gpiod.LineEvent) {
	log.Debugln("end call handler: hangup")
	defer log.Debugln("end call handler: completed hangup")
	if i.station.hasCalls() {
		i.hangup()
	} else {
		i.toggleDoNotDisturb()
	}
}

func (i *physicalInputs) Close() {
	log.Debugln("physicalInputs.Close: enter")
	i.endCallButton.Close()
	log.Debugln("physicalInputs.Closed endCallButton")
	i.groupCallButton.Close()
	log.Debugln("physicalInputs.Closed groupCallButton")
}

func (i *physicalInputs) acceptCall() {
}

func (i *physicalInputs) placeCall(to []string) {
}

func (i *physicalInputs) callAll() {
	log.Debugln("physicalInputs.callAll: enter")
	defer log.Debugln("physicalInputs.callAll: exit")
	i.station.callAll(i.ctx)
}

func (i *physicalInputs) hangup() {
	i.station.hangup()
}

func (i *physicalInputs) setVolume(percent int) {
}

func (i *physicalInputs) setDoNotDisturb(v bool) {
}

func (i *physicalInputs) toggleDoNotDisturb() {
	i.station.Status.Toggle(statusDoNotDisturb)
}
