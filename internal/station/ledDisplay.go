package station

import (
	"context"
	"log"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/warthog618/gpiod"
)

type ledDisplay struct {
	greenLed, yellowLed *gpiod.Line
}

func newLedDisplay(chip *gpiod.Chip) *ledDisplay {
	// Find out which LED pins we're working with on this device
	dotEnv, err := godotenv.Read()
	if err != nil {
		panic(err)
	}

	greenLedPin, _ := strconv.Atoi(dotEnv["GREEN_LED_PIN"])
	log.Printf("Found pin %d for green LED in .env ...\n", greenLedPin)
	yellowLedPin, _ := strconv.Atoi(dotEnv["YELLOW_LED_PIN"])
	log.Printf("Found pin %d for yellow LED in .env ...\n", yellowLedPin)

	// Set up LED lines
	greenLed, err := chip.RequestLine(greenLedPin, gpiod.AsOutput(0))
	if err != nil {
		panic(err)
	}
	yellowLed, err := chip.RequestLine(yellowLedPin, gpiod.AsOutput(0))
	if err != nil {
		panic(err)
	}
	return &ledDisplay{
		greenLed:  greenLed,
		yellowLed: yellowLed,
	}
}

func (d *ledDisplay) IncomingCall(ctx context.Context, from string) {
	d.UpdateStatus(statusIncomingCall)
}

func (d *ledDisplay) UpdateStatus(status int) error {
	if status == statusDefault {
		log.Println("default status")
		err := d.yellowLed.SetValue(0)
		if err != nil {
			return err
		}
		err = d.greenLed.SetValue(0)
		if err != nil {
			return err
		}
	} else if status == statusDoNotDisturb {
		log.Println("do not disturb status")
	} else if status == statusIncomingCall {
		log.Println("incoming call status")
	} else if status == statusOutgoingCall {
		log.Println("outgoing call status")
		d.greenLed.SetValue(1)
	} else if status == statusCallConnected {
		log.Println("call connected status")
		d.greenLed.SetValue(1)
	} else {
		log.Println("Unknown status")
	}
	return nil
}

func (d *ledDisplay) OutgoingCall(ctx context.Context, to string) error {
	err := d.UpdateStatus(statusOutgoingCall)
	return err
}

func (d *ledDisplay) Close() {
	d.greenLed.Close()
	d.yellowLed.Close()
}
