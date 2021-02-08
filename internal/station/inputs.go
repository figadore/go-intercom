package station

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/pkg/call"
	"github.com/warthog618/gpiod"
)

// E.g. buttons, stateful menu with display, voice commands
type physicalInputs struct {
	groupCallButton, endCallButton *gpiod.Line
	// volumeControl                  *struct{}
}

// map of button handlers, take gpiod.LineEvent input
// TODO or should this be a struct?
// also, what creates it? it should be wrappers around handlers in calls/, like callAll, and endCall, (and set dnd?)
type Handlers map[string]func(gpiod.LineEvent)

func newPhysicalInputs(ctx context.Context, chip *gpiod.Chip, dotEnv map[string]string, callManager call.Manager) *physicalInputs {
	redButtonPin, _ := strconv.Atoi(dotEnv["RED_BUTTON_PIN"])
	log.Printf("Found pin %d for red button in .env ...\n", redButtonPin)
	blackButtonPin, _ := strconv.Atoi(dotEnv["BLACK_BUTTON_PIN"])
	log.Printf("Found pin %d for black button in .env ...\n", blackButtonPin)
	// Set up button lines
	input := &physicalInputs{}
	groupCallButton, err := chip.RequestLine(blackButtonPin,
		gpiod.WithDebounce(time.Millisecond*30),
		gpiod.WithFallingEdge, // use WithBothEdges and a timer if long-press required
		gpiod.WithEventHandler(func(_ gpiod.LineEvent) {
			log.Debugln("group call handler: callAll")
			defer log.Debugln("group call handler: completed callAll")
			input.callAll(ctx, callManager)
		}))
	if err != nil {
		msg := fmt.Sprintf("RequestLine returned error: %s\n", err)
		panic(msg)
	}
	endCallButton, err := chip.RequestLine(redButtonPin,
		gpiod.WithDebounce(time.Millisecond*30),
		gpiod.WithFallingEdge,
		gpiod.WithEventHandler(func(_ gpiod.LineEvent) {
			log.Debugln("end call handler: hangup")
			defer log.Debugln("end call handler: completed hangup")
			input.hangup(callManager)
		}))
	if err != nil {
		msg := fmt.Sprintf("RequestLine returned error: %s\n", err)
		log.Println(msg)
		panic(msg)
	}
	input.endCallButton = endCallButton
	input.groupCallButton = groupCallButton
	return input
}

func (i *physicalInputs) Close() {
	i.endCallButton.Close()
	i.groupCallButton.Close()
}

func (i *physicalInputs) acceptCall(ctx context.Context, from string, callManager call.Manager) {
}

func (i *physicalInputs) placeCall(ctx context.Context, to []string, callManager call.Manager) {
}

func (i *physicalInputs) callAll(ctx context.Context, callManager call.Manager) {
	// TODO set station output status?
	log.Debugln("physicalInputs.callAll: enter")
	defer log.Debugln("physicalInputs.callAll: exit")
	callManager.CallAll(ctx)
}

func (i *physicalInputs) hangup(callManager call.Manager) {
	// TODO set station output status?
	// TODO if no call active, toggle do not disturb
	callManager.EndCalls()
}

func (i *physicalInputs) setVolume(percent int) {
}

func (i *physicalInputs) setDoNotDisturb(status int) {
}
