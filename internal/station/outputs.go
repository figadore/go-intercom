package station

import (
	"log"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/warthog618/gpiod"
)

// Allow various ways to display status and other info
// E.g. LEDs, TFT, text to speech
// If UpdateStatus has an error, there is likely nothing to do about it except log it, so no error object is returned
type Outputs interface {
	UpdateStatus(status *Status)
	Close()
}

type led struct {
	line     *gpiod.Line
	ticker   *time.Ticker
	done     chan bool
	blinking bool
}

func newLed(line *gpiod.Line) *led {
	ticker := time.NewTicker(time.Second)
	ticker.Stop()
	return &led{
		line:     line,
		ticker:   ticker,
		done:     make(chan bool),
		blinking: false,
	}
}

func (l *led) blink(interval time.Duration) {
	l.ticker.Reset(interval)
	go func() {
		v := 0
		l.blinking = true
		log.Println("LED blink: waiting for done or ticker")
		for {
			select {
			case <-l.done:
				l.blinking = false
				return
			case <-l.ticker.C:
				v = 1 - v
				log.Println("LED ticker.tick, setting value:", v)
				err := l.line.SetValue(v)
				if err != nil {
					log.Println("Error turning on LED:", err)
				}
			}
		}
	}()
}

func (l *led) on() {
	if l.blinking {
		l.ticker.Stop()
		l.done <- true
		l.blinking = false
	}
	err := l.line.SetValue(1)
	if err != nil {
		log.Println("Error turning on LED:", err)
	}
}

func (l *led) off() {
	if l.blinking {
		l.ticker.Stop()
		l.done <- true
		l.blinking = false
	}
	err := l.line.SetValue(0)
	if err != nil {
		log.Println("Error turning off LED:", err)
	}
}

func (l *led) Close() {
	l.off()
	l.line.Close()
}

type ledDisplay struct {
	greenLed, yellowLed *led
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
		greenLed:  newLed(greenLed),
		yellowLed: newLed(yellowLed),
	}
}

func (d *ledDisplay) UpdateStatus(status *Status) {
	log.Println("Updating LED status")
	d.yellowLed.off()
	d.greenLed.off()
	if status.Has(StatusDoNotDisturb) && !status.Has(StatusCallConnected) {
		// yellow solid
		d.yellowLed.on()
		log.Println("do not disturb status")
	}
	if status.Has(StatusIncomingCall) {
		d.greenLed.blink(time.Millisecond * 500)
		log.Println("incoming call status")
	}
	if status.Has(StatusOutgoingCall) {
		// yellow blink
		log.Println("outgoing call status")
		d.yellowLed.blink(time.Millisecond * 500)
	}
	if status.Has(StatusCallConnected) {
		// green solid
		log.Println("call connected status")
		d.greenLed.on()
	}
	if status.Has(StatusError) {
		// green/yellow on
		d.yellowLed.on()
		d.greenLed.on()
	}
}

func (d *ledDisplay) Close() {
	d.greenLed.Close()
	d.yellowLed.Close()
}
