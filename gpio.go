package main

import (
	"log"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/warthog618/gpiod"

	"github.com/figadore/go-intercom/pb"
)

// Attach event handler to each button
func initializeButtons(eventHandler func(gpiod.LineEvent)) *gpiod.Lines {
	chip, err := gpiod.NewChip("gpiochip0")
	if err != nil {
		panic(err)
	}

	// Find out which pins we're working with
	dotEnv, err = godotenv.Read()
	if err != nil {
		panic(err)
	}
	redButtonPin, _ := strconv.Atoi(dotEnv["RED_BUTTON_PIN"])
	log.Printf("Found pin %d for red button in .env ...\n", redButtonPin)
	blackButtonPin, _ := strconv.Atoi(dotEnv["BLACK_BUTTON_PIN"])
	log.Printf("Found pin %d for black button in .env ...\n", blackButtonPin)

	// Set up button lines
	lines, err := chip.RequestLines([]int{redButtonPin, blackButtonPin},
		gpiod.WithDebounce(time.Millisecond*30),
		gpiod.WithBothEdges,
		gpiod.WithEventHandler(eventHandler))
	if err != nil {
		log.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			log.Println("Note that the WithDebounce option requires kernel V5.10 or later - check your kernel version.")
		}
		os.Exit(1)
	}
	log.Println("requested both lines")
	return lines
}

func initializeLeds(chip *gpiod.Chip) {
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
	greenLed, err = chip.RequestLine(greenLedPin, gpiod.AsOutput(0))
	if err != nil {
		panic(err)
	}
	yellowLed, err = chip.RequestLine(yellowLedPin, gpiod.AsOutput(0))
	if err != nil {
		panic(err)
	}
}

func updateLeds(stateChange *pb.ButtonStateChange) error {
	var err error
	if stateChange.GetButton() == "black" {
		if stateChange.GetPressed() {
			err = greenLed.SetValue(1)
		} else {
			err = greenLed.SetValue(0)
		}
	} else {
		if stateChange.GetPressed() {
			err = yellowLed.SetValue(1)
		} else {
			err = yellowLed.SetValue(0)
		}
	}
	return err
}
