package station

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"syscall"
	"time"

	"github.com/figadore/go-intercom/pkg/call"
	"github.com/warthog618/gpiod"
)

// E.g. buttons, stateful menu with display, voice commands
type physicalInputs struct {
	groupCallButton, endCallButton *gpiod.Line
	volumeControl                  *struct{}
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
	groupCallButton, err := chip.RequestLine(blackButtonPin,
		gpiod.WithDebounce(time.Millisecond*30),
		gpiod.WithFallingEdge, // use WithBothEdges and a timer if long-press required
		gpiod.WithEventHandler(func(_ gpiod.LineEvent) { callManager.CallAll(ctx) }))
	if err != nil {
		msg := fmt.Sprintf("RequestLine returned error: %s\n", err)
		panic(msg)
	}
	endCallButton, err := chip.RequestLine(redButtonPin,
		gpiod.WithDebounce(time.Millisecond*30),
		gpiod.WithFallingEdge,
		gpiod.WithEventHandler(func(_ gpiod.LineEvent) { callManager.EndCalls() }))
	if err != nil {
		msg := fmt.Sprintf("RequestLine returned error: %s\n", err)
		log.Println(msg)
		panic(msg)
	}
	return &physicalInputs{
		endCallButton:   endCallButton,
		groupCallButton: groupCallButton,
	}
}

func (i *physicalInputs) Close() {
	i.endCallButton.Close()
	i.groupCallButton.Close()
}

func (i *physicalInputs) acceptCall(ctx context.Context, from string) {
}

func (i *physicalInputs) placeCall(ctx context.Context, to []string) {
}

func (i *physicalInputs) hangup() {
}

func (i *physicalInputs) setVolume(percent int) {
}

func (i *physicalInputs) setDoNotDisturb(status int) {
}

type ButtonWithHandler struct {
	button  *gpiod.Line
	handler func(gpiod.LineEvent)
}

func (*physicalInputs) setEventHandlers(buttonsWithHandlers []ButtonWithHandler) {
	for _, buttonWithHandler := range buttonsWithHandlers {
		var chip gpiod.Chip
		_, err := chip.RequestLine(3,
			gpiod.WithDebounce(time.Millisecond*30),
			gpiod.WithBothEdges,
			gpiod.WithEventHandler(buttonWithHandler.handler))
		if err != nil {
			msg := fmt.Sprintf("RequestLine returned error: %s\n", err)
			if err == syscall.Errno(22) {
				log.Println("Note that the WithDebounce option requires kernel V5.10 or later - check your kernel version.")
			}
			panic(msg)
		}
	}
}
